package com.banka1.interbank.service;

import com.banka1.interbank.model.InterbankMessageEntity;
import com.banka1.interbank.model.enums.Direction;
import com.banka1.interbank.model.enums.MessageStatus;
import com.banka1.interbank.protocol.dto.InterbankMessagePayload;
import com.banka1.interbank.repository.InterbankMessageRepository;
import com.fasterxml.jackson.databind.ObjectMapper;
import java.util.Optional;
import lombok.RequiredArgsConstructor;
import lombok.extern.slf4j.Slf4j;
import org.springframework.stereotype.Service;

/**
 * PR_32 Phase 7 Task 7.1: idempotency cache za INBOUND poruke.
 *
 * <p>Posle uspesne obrade {@link com.banka1.interbank.controller.InterbankInboundController}-a
 * pozove {@link #persistInbound(InterbankMessagePayload, int, String)} sa HTTP statusom +
 * serialized response body-jem. Naredni retry istog (sender_routing, locally_generated_key)
 * para dobija cached response umesto da re-obradi transakciju (per Tim 2 §2.2).
 *
 * <p>Unique constraint na {@code (direction, sender_routing_number, locally_generated_key)} je
 * garantovan u {@link InterbankMessageEntity} schema-i — paralelni retry-i na isti key bice
 * zavrseni od strane prvog committed insert-a, a drugi ce dobici cached response u sledecem
 * request-u.
 */
@Service
@RequiredArgsConstructor
@Slf4j
public class InterbankMessageService {

    private final InterbankMessageRepository repo;
    private final ObjectMapper mapper;

    /**
     * Lookup po (direction, sender_routing, locally_generated_key).
     *
     * @param dir           direkcija poruke; obicno {@link Direction#INBOUND} za controller use-case
     * @param senderRouting routing number partner-a koji je poslao poruku
     * @param key           locally-generated key iz {@code idempotenceKey}
     * @return Optional sa cached entity-jem ako je vec procesirana, prazan Optional inace
     */
    public Optional<InterbankMessageEntity> findCached(Direction dir, int senderRouting, String key) {
        return repo.findByDirectionAndSenderRoutingNumberAndLocallyGeneratedKey(dir, senderRouting, key);
    }

    /**
     * Persist successful INBOUND poruke u idempotency cache. Posle commit-a, retry istog key-a
     * vraca cached HTTP status + response body bez ponovne obrade.
     *
     * @param msg          original payload (od strane partner banke)
     * @param httpStatus   HTTP status koji controller vraca (npr. 200, 204)
     * @param responseBody serialized response body; {@code null} dozvoljeno za 204 No Content
     * @throws RuntimeException ako se request body ne moze serijalizovati u JSON (mapper greska)
     */
    public void persistInbound(InterbankMessagePayload msg, int httpStatus, String responseBody) {
        try {
            var entity = InterbankMessageEntity.builder()
                    .direction(Direction.INBOUND)
                    .senderRoutingNumber(msg.idempotenceKey().routingNumber())
                    .locallyGeneratedKey(msg.idempotenceKey().locallyGeneratedKey())
                    .messageType(msg.messageType())
                    .status(MessageStatus.INBOUND_PROCESSED)
                    .requestBody(mapper.writeValueAsString(msg))
                    .responseBody(responseBody)
                    .httpStatus(httpStatus)
                    .retryCount(0)
                    .build();
            repo.save(entity);
        } catch (Exception e) {
            log.error("Failed to persist idempotency cache for {}", msg.idempotenceKey(), e);
            throw new RuntimeException(e);
        }
    }
}
