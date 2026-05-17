package com.banka1.interbank.service;

import com.banka1.interbank.config.InterbankProperties;
import com.banka1.interbank.model.InterbankMessageEntity;
import com.banka1.interbank.model.enums.Direction;
import com.banka1.interbank.model.enums.MessageStatus;
import com.banka1.interbank.otc.dto.OtcOfferDto;
import com.banka1.interbank.protocol.dto.CommitTransactionBody;
import com.banka1.interbank.protocol.dto.ForeignBankId;
import com.banka1.interbank.protocol.dto.IdempotenceKey;
import com.banka1.interbank.protocol.dto.InterbankMessagePayload;
import com.banka1.interbank.protocol.dto.InterbankTransactionPayload;
import com.banka1.interbank.protocol.dto.MessageType;
import com.banka1.interbank.protocol.dto.RollbackTransactionBody;
import com.banka1.interbank.protocol.dto.TransactionVote;
import com.banka1.interbank.repository.InterbankMessageRepository;
import com.fasterxml.jackson.databind.ObjectMapper;
import java.security.SecureRandom;
import java.time.Instant;
import java.util.HexFormat;
import lombok.extern.slf4j.Slf4j;
import org.springframework.beans.factory.annotation.Qualifier;
import org.springframework.context.annotation.Profile;
import org.springframework.http.ResponseEntity;
import org.springframework.stereotype.Component;
import org.springframework.transaction.annotation.Transactional;
import org.springframework.web.client.HttpClientErrorException;
import org.springframework.web.client.RestClient;

/**
 * PR_32 Phase 8 Task 8.2: OUTBOUND HTTP klijent za interbank protokol (per Tim 2 §6.4).
 *
 * <p>Tri javne metode formiraju 2PC tok ka partnerskoj banci:
 * <ul>
 *   <li>{@link #sendNewTx(int, InterbankTransactionPayload)} — prepare faza, vraca
 *       {@link TransactionVote} (YES/NO).</li>
 *   <li>{@link #sendCommitTx(int, ForeignBankId)} — commit faza, vraca 204 No Content.</li>
 *   <li>{@link #sendRollbackTx(int, ForeignBankId)} — rollback faza, vraca 204 No Content.</li>
 * </ul>
 *
 * <p>Pre svakog HTTP poziva, poruka se persistira u {@code interbank_messages} sa statusom
 * {@link MessageStatus#PENDING_SEND}. Posle uspesnog 2xx odgovora, status prelazi u
 * {@link MessageStatus#SENT}. Pri exception-u poruka ostaje u PENDING_SEND i biva pickovana
 * od {@link com.banka1.interbank.scheduler.InterbankRetryScheduler}-a u sledecem ciklusu.
 *
 * <p>Idempotency key se generise lokalno (16 random bajtova → hex) sa {@code routingNumber =
 * mojRoutingNumber}. Partner banka ga koristi za idempotency cache na svojoj strani — retry
 * istog (routingNumber, locallyGeneratedKey) vraca cached response.
 *
 * <p>{@link #resendByEntity(InterbankMessageEntity)} se zove iz retry scheduler-a i reuse-uje
 * isti entity (i isti locallyGeneratedKey) — partner ce videti retry kao idempotentan.
 */
@Component
@Profile("!test")
@Slf4j
public class InterbankClient {

    private final RestClient client;
    private final InterbankProperties props;
    private final InterbankMessageRepository msgRepo;
    private final ObjectMapper mapper;
    private final SecureRandom random = new SecureRandom();

    public InterbankClient(@Qualifier("interbankOutboundRestClient") RestClient client,
                           InterbankProperties props,
                           InterbankMessageRepository msgRepo,
                           ObjectMapper mapper) {
        this.client = client;
        this.props = props;
        this.msgRepo = msgRepo;
        this.mapper = mapper;
    }

    /**
     * Posalji NEW_TX (prepare) ka partner banci. Sinhrono blokira do odgovora ili timeout-a
     * (20s read timeout iz {@link com.banka1.interbank.config.RestClientConfig}).
     *
     * @param partnerRouting routing broj partner banke
     * @param tx             payload transakcije (postings, transactionId, etc.)
     * @return {@link TransactionVote} (YES sa null reasons, ili NO sa listom razloga)
     */
    public TransactionVote sendNewTx(int partnerRouting, InterbankTransactionPayload tx) {
        IdempotenceKey key = newKey();
        InterbankMessagePayload payload = new InterbankMessagePayload(
                key, MessageType.NEW_TX, mapper.valueToTree(tx));
        InterbankMessageEntity logEntry = persistOutboundPending(partnerRouting, payload);

        InterbankProperties.Partner partner = props.partnerOrThrow(partnerRouting);
        TransactionVote vote = client.post()
                .uri(partner.getBaseUrl() + "interbank")
                .header("X-Api-Key", partner.getOutboundToken())
                .body(payload)
                .retrieve()
                .body(TransactionVote.class);

        try {
            markSent(logEntry, 200, mapper.writeValueAsString(vote));
        } catch (Exception e) {
            throw new RuntimeException("Cannot serialize TransactionVote response", e);
        }
        return vote;
    }

    /**
     * Posalji COMMIT_TX (2PC commit) ka partner banci. Ocekuje se 204 No Content.
     *
     * @param partnerRouting routing broj partner banke
     * @param txId           foreign bank id transakcije (routing + lokalni id inicijatora)
     */
    public void sendCommitTx(int partnerRouting, ForeignBankId txId) {
        IdempotenceKey key = newKey();
        InterbankMessagePayload payload = new InterbankMessagePayload(
                key, MessageType.COMMIT_TX, mapper.valueToTree(new CommitTransactionBody(txId)));
        InterbankMessageEntity logEntry = persistOutboundPending(partnerRouting, payload);

        InterbankProperties.Partner partner = props.partnerOrThrow(partnerRouting);
        client.post()
                .uri(partner.getBaseUrl() + "interbank")
                .header("X-Api-Key", partner.getOutboundToken())
                .body(payload)
                .retrieve()
                .toBodilessEntity();

        markSent(logEntry, 204, null);
    }

    /**
     * Posalji ROLLBACK_TX (2PC rollback) ka partner banci. Ocekuje se 204 No Content.
     *
     * @param partnerRouting routing broj partner banke
     * @param txId           foreign bank id transakcije
     */
    public void sendRollbackTx(int partnerRouting, ForeignBankId txId) {
        IdempotenceKey key = newKey();
        InterbankMessagePayload payload = new InterbankMessagePayload(
                key, MessageType.ROLLBACK_TX, mapper.valueToTree(new RollbackTransactionBody(txId)));
        InterbankMessageEntity logEntry = persistOutboundPending(partnerRouting, payload);

        InterbankProperties.Partner partner = props.partnerOrThrow(partnerRouting);
        client.post()
                .uri(partner.getBaseUrl() + "interbank")
                .header("X-Api-Key", partner.getOutboundToken())
                .body(payload)
                .retrieve()
                .toBodilessEntity();

        markSent(logEntry, 204, null);
    }

    /**
     * PR_33 Phase A: outbound POST /negotiations ka partner banci.
     *
     * <p>Saljemo {@link OtcOfferDto} sa {@code X-Api-Key} outbound tokenom.
     * Partner persistuje pregovor i vraca {@link ForeignBankId} svog
     * routing broja + novi negotiation id. NE persistira se u
     * {@code interbank_messages} tabelu — to je za 2PC mesigning samo
     * (NEW_TX/COMMIT_TX/ROLLBACK_TX). OTC §3 rute su zasebne (Tim 2 §3 vs
     * §6.4 podela).
     *
     * @param partnerRouting routing broj partner banke
     * @param offer          payload ponude (mora imati
     *                       {@code lastModifiedBy = {myRouting, "C-..."}})
     * @return ForeignBankId vracen od partner-a (njegov routing + negotiation id)
     */
    public ForeignBankId outboundCreateNegotiation(int partnerRouting, OtcOfferDto offer) {
        InterbankProperties.Partner partner = props.partnerOrThrow(partnerRouting);
        ForeignBankId id = client.post()
                .uri(partner.getBaseUrl() + "negotiations")
                .header("X-Api-Key", partner.getOutboundToken())
                .body(offer)
                .retrieve()
                .body(ForeignBankId.class);
        log.info("Outbound POST /negotiations partner={} returned id={}", partnerRouting, id);
        return id;
    }

    /**
     * PR_33 Phase A: outbound PUT /negotiations/{rn}/{id} ka partner banci
     * (counter-offer iz nase strane).
     *
     * <p>Per Tim 2 §6.3, partner moze vratiti 204 / 400 / 404 / 409.
     * Propagiramo originalni status nazad pozivocu (kontroler vraca FE-u
     * istu vrednost).
     *
     * @param partnerRouting routing broj partner banke
     * @param negId          authoritative id pregovora (partner routing + partner id)
     * @param offer          counter-offer payload
     * @return ResponseEntity sa status code-om iz partner-a (204 happy, 4xx error)
     */
    public ResponseEntity<Void> outboundPutCounter(int partnerRouting,
                                                   ForeignBankId negId,
                                                   OtcOfferDto offer) {
        InterbankProperties.Partner partner = props.partnerOrThrow(partnerRouting);
        try {
            return client.put()
                    .uri(partner.getBaseUrl() + "negotiations/" + negId.routingNumber()
                            + "/" + negId.id())
                    .header("X-Api-Key", partner.getOutboundToken())
                    .body(offer)
                    .retrieve()
                    .toBodilessEntity();
        } catch (HttpClientErrorException e) {
            // 400 / 404 / 409 — propagate status code, swallow body (FE dobija
            // status iz nas controller-a + clean error message kroz exception handler).
            log.warn("Outbound PUT /negotiations/{}/{} partner returned {}: {}",
                    negId.routingNumber(), negId.id(), e.getStatusCode(),
                    e.getResponseBodyAsString());
            return ResponseEntity.status(e.getStatusCode()).build();
        }
    }

    /**
     * PR_33 Phase A: outbound GET /negotiations/{rn}/{id}/accept ka partner banci.
     *
     * <p>Mi smo buyer-bank i prihvatamo seller-ovu ponudu. Partner pokrece
     * 2PC kroz svoj coordinator (njegov accept endpoint moze drzati konekciju
     * do 60s per Tim 2 §6.6). Ako 2PC uspeva, partner vraca 204 No Content
     * — to znaci da je COMMIT_TX kroz partner-ov coordinator stigao do nas
     * (kroz nas {@link com.banka1.interbank.controller.InterbankInboundController}-a)
     * i da je nas commit local takodje uspeo.
     *
     * @param partnerRouting routing broj partner banke
     * @param negId          authoritative id pregovora
     * @return ResponseEntity sa status code-om iz partner-a
     */
    public ResponseEntity<Void> outboundAccept(int partnerRouting, ForeignBankId negId) {
        InterbankProperties.Partner partner = props.partnerOrThrow(partnerRouting);
        try {
            return client.get()
                    .uri(partner.getBaseUrl() + "negotiations/" + negId.routingNumber()
                            + "/" + negId.id() + "/accept")
                    .header("X-Api-Key", partner.getOutboundToken())
                    .retrieve()
                    .toBodilessEntity();
        } catch (HttpClientErrorException e) {
            log.warn("Outbound GET /negotiations/{}/{}/accept partner returned {}",
                    negId.routingNumber(), negId.id(), e.getStatusCode());
            return ResponseEntity.status(e.getStatusCode()).build();
        }
    }

    /**
     * PR_33 Phase A: outbound DELETE /negotiations/{rn}/{id} ka partner banci.
     * Idempotent: partner vraca 204 i ako je pregovor vec zatvoren.
     *
     * @param partnerRouting routing broj partner banke
     * @param negId          authoritative id pregovora
     * @return ResponseEntity sa status code-om iz partner-a
     */
    public ResponseEntity<Void> outboundDelete(int partnerRouting, ForeignBankId negId) {
        InterbankProperties.Partner partner = props.partnerOrThrow(partnerRouting);
        try {
            return client.delete()
                    .uri(partner.getBaseUrl() + "negotiations/" + negId.routingNumber()
                            + "/" + negId.id())
                    .header("X-Api-Key", partner.getOutboundToken())
                    .retrieve()
                    .toBodilessEntity();
        } catch (HttpClientErrorException e) {
            log.warn("Outbound DELETE /negotiations/{}/{} partner returned {}",
                    negId.routingNumber(), negId.id(), e.getStatusCode());
            return ResponseEntity.status(e.getStatusCode()).build();
        }
    }

    /**
     * PR_33 follow-up: outbound GET /public-stock ka partner banci.
     * Frontend wrapper koristi za "Banka 2 javne akcije" discovery view.
     *
     * @param partnerRouting routing broj partner banke
     * @return JSON niz {@link com.banka1.interbank.otc.dto.PublicStockEntryDto},
     *         ili prazna lista pri 4xx/5xx (graceful — UI tabela bude prazna).
     */
    public java.util.List<com.banka1.interbank.otc.dto.PublicStockEntryDto> outboundFetchPublicStock(int partnerRouting) {
        InterbankProperties.Partner partner = props.partnerOrThrow(partnerRouting);
        try {
            com.banka1.interbank.otc.dto.PublicStockEntryDto[] body = client.get()
                    .uri(partner.getBaseUrl() + "public-stock")
                    .header("X-Api-Key", partner.getOutboundToken())
                    .retrieve()
                    .body(com.banka1.interbank.otc.dto.PublicStockEntryDto[].class);
            return body != null ? java.util.Arrays.asList(body) : java.util.List.of();
        } catch (HttpClientErrorException e) {
            log.warn("Outbound GET /public-stock to partner {} returned {}",
                    partnerRouting, e.getStatusCode());
            return java.util.List.of();
        } catch (RuntimeException e) {
            log.warn("Outbound GET /public-stock to partner {} failed: {}",
                    partnerRouting, e.getMessage());
            return java.util.List.of();
        }
    }

    /**
     * Re-send postojece poruke (poziva se iz {@link
     * com.banka1.interbank.scheduler.InterbankRetryScheduler}-a). Reuse-uje
     * isti locallyGeneratedKey iz original entity-ja tako da partner vidi retry kao
     * idempotentan poziv.
     *
     * <p>Razlikuje 200 OK (NEW_TX response sa TransactionVote body-jem) od 204 No Content
     * (COMMIT_TX / ROLLBACK_TX) preko {@link MessageType}-a u original payload-u.
     *
     * @param entity originalna persistirana poruka u PENDING_SEND ili SENT statusu
     */
    public void resendByEntity(InterbankMessageEntity entity) {
        InterbankMessagePayload payload;
        try {
            payload = mapper.readValue(entity.getRequestBody(), InterbankMessagePayload.class);
        } catch (Exception e) {
            throw new RuntimeException(
                    "Cannot parse persisted request body for message id=" + entity.getId(), e);
        }

        int partnerRouting = entity.getSenderRoutingNumber();
        InterbankProperties.Partner partner = props.partnerOrThrow(partnerRouting);

        if (entity.getMessageType() == MessageType.NEW_TX) {
            TransactionVote vote = client.post()
                    .uri(partner.getBaseUrl() + "interbank")
                    .header("X-Api-Key", partner.getOutboundToken())
                    .body(payload)
                    .retrieve()
                    .body(TransactionVote.class);
            try {
                markSent(entity, 200, mapper.writeValueAsString(vote));
            } catch (Exception e) {
                throw new RuntimeException("Cannot serialize TransactionVote response", e);
            }
        } else {
            client.post()
                    .uri(partner.getBaseUrl() + "interbank")
                    .header("X-Api-Key", partner.getOutboundToken())
                    .body(payload)
                    .retrieve()
                    .toBodilessEntity();
            markSent(entity, 204, null);
        }
    }

    /**
     * Generise novi {@link IdempotenceKey} sa nasim routing brojem i 16 random bajtova
     * (32 hex karaktera) kao locallyGeneratedKey.
     */
    IdempotenceKey newKey() {
        byte[] bytes = new byte[16];
        random.nextBytes(bytes);
        return new IdempotenceKey(props.getMyRoutingNumber(), HexFormat.of().formatHex(bytes));
    }

    /**
     * Persist nove OUTBOUND poruke u {@link MessageStatus#PENDING_SEND}.
     *
     * <p>{@code senderRoutingNumber} cuva routing PARTNERA (primaoca) — to omogucava da
     * retry scheduler zna kome posalje retry bez parsiranja request body-ja.
     *
     * <p>Throws {@link RuntimeException} ako se payload ne moze serijalizovati u JSON
     * (mapper greska). Jackson serializacija {@code @NotNull JsonNode message} ne baca
     * IOException pa je catch defenzivan.
     *
     * @param partnerRouting routing broj primaoca (partner banke)
     * @param payload        kompletni message payload
     * @return persistirana entity sa ID-jem
     */
    @Transactional
    InterbankMessageEntity persistOutboundPending(int partnerRouting, InterbankMessagePayload payload) {
        try {
            return msgRepo.save(InterbankMessageEntity.builder()
                    .direction(Direction.OUTBOUND)
                    .senderRoutingNumber(partnerRouting)
                    .locallyGeneratedKey(payload.idempotenceKey().locallyGeneratedKey())
                    .messageType(payload.messageType())
                    .status(MessageStatus.PENDING_SEND)
                    .requestBody(mapper.writeValueAsString(payload))
                    .lastAttemptAt(Instant.now())
                    .retryCount(0)
                    .build());
        } catch (Exception e) {
            throw new RuntimeException("Cannot persist outbound message", e);
        }
    }

    /**
     * Update postojece poruke u {@link MessageStatus#SENT} sa HTTP status code-om i
     * response body-jem (moze biti null za 204).
     */
    @Transactional
    void markSent(InterbankMessageEntity entity, int httpStatus, String responseBody) {
        entity.setStatus(MessageStatus.SENT);
        entity.setHttpStatus(httpStatus);
        entity.setResponseBody(responseBody);
        entity.setLastAttemptAt(Instant.now());
        msgRepo.save(entity);
    }
}
