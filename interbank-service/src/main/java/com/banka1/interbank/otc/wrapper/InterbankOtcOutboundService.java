package com.banka1.interbank.otc.wrapper;

import com.banka1.interbank.config.InterbankProperties;
import com.banka1.interbank.exception.InvalidNegotiationException;
import com.banka1.interbank.exception.NegotiationClosedException;
import com.banka1.interbank.exception.NegotiationNotFoundException;
import com.banka1.interbank.exception.TurnViolationException;
import com.banka1.interbank.model.InterbankNegotiationEntity;
import com.banka1.interbank.otc.dto.OtcNegotiationDto;
import com.banka1.interbank.otc.dto.OtcOfferDto;
import com.banka1.interbank.otc.wrapper.dto.OutboundCounterOfferRequest;
import com.banka1.interbank.otc.wrapper.dto.OutboundCreateNegotiationRequest;
import com.banka1.interbank.otc.wrapper.dto.OutboundNegotiationResponse;
import com.banka1.interbank.protocol.dto.CurrencyCode;
import com.banka1.interbank.protocol.dto.ForeignBankId;
import com.banka1.interbank.protocol.dto.MonetaryValue;
import com.banka1.interbank.protocol.dto.StockDescription;
import com.banka1.interbank.repository.InterbankNegotiationRepository;
import com.banka1.interbank.service.InterbankClient;
import com.banka1.interbank.service.OtcNegotiationService;
import java.security.SecureRandom;
import java.time.OffsetDateTime;
import java.util.HexFormat;
import java.util.List;
import lombok.RequiredArgsConstructor;
import lombok.extern.slf4j.Slf4j;
import org.springframework.http.ResponseEntity;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;

/**
 * PR_33 Phase A: business logic za FE-facing /api/interbank/otc/* rute.
 *
 * <p>Analogno Tim 2's {@code InterbankOtcWrapperController} — wrapper koji
 * prevodi UI-friendly DTO-ove u protokolarne {@link OtcOfferDto} i obrnuto,
 * vodeci racuna o:
 * <ul>
 *   <li>Routing brojevima: nas (buyer) routing = {@code myRouting}, seller
 *       routing = partner-ov.</li>
 *   <li>User ID prefix konvenciji: klijent kod nas postaje
 *       {@code "C-{principalId}"} per Tim 2 §10 prefiks tabeli.</li>
 *   <li>{@code lastModifiedBy} polju: uvek mi (nas routing + nas user id)
 *       kad inicijiramo ili kontriramo.</li>
 *   <li>Local mirror state-u: persist sa
 *       {@code is_authoritative=false}, {@code remote_negotiation_id=partner_id}.</li>
 * </ul>
 *
 * <p><strong>Turn semantika za outbound:</strong> kad MI inicijiramo
 * pregovor, automatski smo poslednji modifikator → seller (partner) sledeci
 * deluje (counter ili accept). Kad MI counter-ujemo, isto. Kad MI accept-ujemo,
 * seller-ov bank pokrece 2PC ka nama (mi smo participant, partner je coordinator).
 */
@Service
@RequiredArgsConstructor
@Slf4j
public class InterbankOtcOutboundService {

    private final InterbankClient interbankClient;
    private final InterbankNegotiationRepository negRepo;
    private final InterbankProperties props;
    private final OtcNegotiationService otcService;
    private final SecureRandom random = new SecureRandom();

    /**
     * POST /api/interbank/otc/negotiations — inicijiraj outbound pregovor.
     *
     * <p>Tok:
     * <ol>
     *   <li>Validacije: seller != mi, settlement u buducnosti, amount/price/premium > 0.</li>
     *   <li>Konstruisi {@link OtcOfferDto} sa {@code buyerId = {myRouting, "C-{userId}"}},
     *       {@code lastModifiedBy = buyerId}.</li>
     *   <li>POST ka partner-u kroz {@link InterbankClient#outboundCreateNegotiation}.</li>
     *   <li>Partner vraca {@link ForeignBankId} (njegov routing + njegov negotiation id).</li>
     *   <li>Persist u nasu {@code interbank_negotiations} tabelu sa
     *       {@code is_authoritative=false}, {@code remote_negotiation_id=partner_id}.</li>
     *   <li>Vrati {@link OutboundNegotiationResponse}.</li>
     * </ol>
     */
    @Transactional
    public OutboundNegotiationResponse createOutbound(OutboundCreateNegotiationRequest req,
                                                      Long buyerLocalUserId) {
        int myRouting = props.getMyRoutingNumber();
        int sellerRouting = req.sellerForeignBankId().routingNumber();
        if (sellerRouting == myRouting) {
            throw new InvalidNegotiationException(
                    "sellerForeignBankId.routingNumber must NOT be my routing (" + myRouting
                            + ") — for intra-bank OTC use /otc endpoints");
        }
        if (req.settlementDate().isBefore(OffsetDateTime.now())) {
            throw new InvalidNegotiationException("settlementDate must be in the future");
        }
        if (req.amount() <= 0) {
            throw new InvalidNegotiationException("amount must be positive");
        }
        if (req.pricePerUnit() == null || req.pricePerUnit().signum() <= 0) {
            throw new InvalidNegotiationException("pricePerUnit must be positive");
        }
        if (req.premium() == null || req.premium().signum() < 0) {
            throw new InvalidNegotiationException("premium must be non-negative");
        }
        if (buyerLocalUserId == null) {
            throw new InvalidNegotiationException(
                    "buyerLocalUserId is null — neither JWT principal nor request body provides it");
        }

        String buyerIdStr = "C-" + buyerLocalUserId;
        ForeignBankId buyer = new ForeignBankId(myRouting, buyerIdStr);
        ForeignBankId seller = req.sellerForeignBankId();

        OtcOfferDto offer = new OtcOfferDto(
                new StockDescription(req.stockTicker()),
                req.settlementDate(),
                new MonetaryValue(req.priceCurrency(), req.pricePerUnit()),
                new MonetaryValue(req.premiumCurrency(), req.premium()),
                buyer,
                seller,
                req.amount(),
                buyer  // mi smo inicijatori, znaci poslednji koji je modifikovao
        );

        ForeignBankId remoteId = interbankClient.outboundCreateNegotiation(sellerRouting, offer);
        if (remoteId == null || remoteId.id() == null || remoteId.id().isBlank()) {
            throw new IllegalStateException(
                    "Partner returned null/blank negotiation id from POST /negotiations");
        }

        String localId = generateNegotiationId();
        InterbankNegotiationEntity entity = InterbankNegotiationEntity.builder()
                .id(localId)
                .buyerRoutingNumber(myRouting)
                .buyerId(buyerIdStr)
                .sellerRoutingNumber(sellerRouting)
                .sellerId(seller.id())
                .stockTicker(req.stockTicker())
                .amount(req.amount())
                .priceCurrency(req.priceCurrency().name())
                .priceAmount(req.pricePerUnit())
                .premiumCurrency(req.premiumCurrency().name())
                .premiumAmount(req.premium())
                .settlementDate(req.settlementDate())
                .lastModifiedByRouting(myRouting)
                .lastModifiedById(buyerIdStr)
                .isOngoing(true)
                .isAuthoritative(false)  // partner je authoritative
                .remoteNegotiationId(remoteId.id())
                .build();
        negRepo.save(entity);
        log.info("Created OUTBOUND interbank negotiation localId={} remoteId={}/{} buyer={}/{}",
                localId, remoteId.routingNumber(), remoteId.id(), myRouting, buyerIdStr);

        return new OutboundNegotiationResponse(localId, remoteId, otcService.toDto(entity));
    }

    /**
     * GET /api/interbank/otc/negotiations — list pregovore u kojima ucestvuje user.
     *
     * <p>Vraca sve {@link InterbankNegotiationEntity} gde je user buyer ili seller
     * sa nasim routing brojem. Admin/SUPERVISOR korisnici dobijaju widen-ovan
     * scope — to se odlucuje na controller nivou kroz {@code includeAll} flag.
     *
     * @param principalLocalId  numeric user id iz JWT-a (može biti null za admin scope)
     * @param includeAll        true za admin/supervisor — vraca SVE inter-bank pregovore
     */
    @Transactional(readOnly = true)
    public List<OutboundNegotiationResponse> listForUser(Long principalLocalId, boolean includeAll) {
        int myRouting = props.getMyRoutingNumber();
        List<InterbankNegotiationEntity> entities;
        if (includeAll) {
            entities = negRepo.findAll();
        } else {
            if (principalLocalId == null) {
                throw new InvalidNegotiationException(
                        "Cannot list — principal id is null and includeAll is false");
            }
            String userIdStr = "C-" + principalLocalId;
            // Pregovori gde sam ja buyer (outbound od mene)
            var asBuyer = negRepo.findByBuyerRoutingNumberAndBuyerId(myRouting, userIdStr);
            // Pregovori gde sam ja seller (inbound)
            var asSeller = negRepo.findBySellerRoutingNumberAndSellerIdAndIsOngoing(
                    myRouting, userIdStr, true);
            entities = new java.util.ArrayList<>(asBuyer);
            // De-dup po id-u
            asSeller.stream()
                    .filter(e -> entities.stream().noneMatch(b -> b.getId().equals(e.getId())))
                    .forEach(entities::add);
        }
        return entities.stream()
                .map(e -> new OutboundNegotiationResponse(
                        e.getId(),
                        remoteForeignBankIdOf(e),
                        otcService.toDto(e)))
                .toList();
    }

    /**
     * GET /api/interbank/otc/negotiations/{id} — fetch state pregovora.
     *
     * <p>{@code id} moze biti:
     * <ul>
     *   <li>Nas lokalni neg-id (npr. {@code neg-ab12cd34}).</li>
     *   <li>Partner-ov remote id ({@code remoteNegotiationId} polje) — iterate.</li>
     * </ul>
     */
    @Transactional(readOnly = true)
    public OutboundNegotiationResponse getOne(String id) {
        InterbankNegotiationEntity entity = findByLocalOrRemoteId(id);
        return new OutboundNegotiationResponse(
                entity.getId(),
                remoteForeignBankIdOf(entity),
                otcService.toDto(entity));
    }

    /**
     * PUT /api/interbank/otc/negotiations/{id}/counter — counter-offer ka partner-u.
     *
     * <p>Tok:
     * <ol>
     *   <li>Load local mirror; mora is_ongoing=true.</li>
     *   <li>Turn check: lastModifiedByRouting != myRouting (drugi je zadnji menjao).</li>
     *   <li>Build {@link OtcOfferDto} sa novim vrednostima, {@code lastModifiedBy =
     *       {myRouting, "C-{principalLocalId}"}}.</li>
     *   <li>PUT ka partner-u kroz {@link InterbankClient#outboundPutCounter}.</li>
     *   <li>Ako partner 204 — update local mirror.</li>
     *   <li>Ako partner 4xx — propagate status nazad FE-u (NE update local).</li>
     * </ol>
     *
     * @return ResponseEntity sa partner-ovim status code-om i (kad happy) updated state-om
     */
    @Transactional
    public ResponseEntity<OutboundNegotiationResponse> counterOutbound(
            String id, OutboundCounterOfferRequest req, Long principalLocalId) {
        if (principalLocalId == null) {
            throw new InvalidNegotiationException(
                    "principal id is null — cannot determine counter-offer initiator");
        }
        int myRouting = props.getMyRoutingNumber();
        InterbankNegotiationEntity entity = findByLocalOrRemoteId(id);

        if (!entity.isOngoing()) {
            throw new NegotiationClosedException("Negotiation " + id + " is closed");
        }
        if (entity.getSettlementDate() != null
                && entity.getSettlementDate().isBefore(OffsetDateTime.now())) {
            throw new NegotiationClosedException("Negotiation " + id + " settlement date passed");
        }
        if (entity.getLastModifiedByRouting() == myRouting) {
            throw new TurnViolationException(
                    "Not your turn — your bank made the last modification");
        }
        if (req.settlementDate().isBefore(OffsetDateTime.now())) {
            throw new InvalidNegotiationException("settlementDate must be in the future");
        }
        if (req.amount() <= 0) {
            throw new InvalidNegotiationException("amount must be positive");
        }
        if (req.pricePerUnit() == null || req.pricePerUnit().signum() <= 0) {
            throw new InvalidNegotiationException("pricePerUnit must be positive");
        }
        if (req.premium() == null || req.premium().signum() < 0) {
            throw new InvalidNegotiationException("premium must be non-negative");
        }

        String myUserIdStr = "C-" + principalLocalId;
        ForeignBankId modifier = new ForeignBankId(myRouting, myUserIdStr);
        ForeignBankId buyer = new ForeignBankId(entity.getBuyerRoutingNumber(), entity.getBuyerId());
        ForeignBankId seller = new ForeignBankId(entity.getSellerRoutingNumber(), entity.getSellerId());

        OtcOfferDto offer = new OtcOfferDto(
                new StockDescription(entity.getStockTicker()),
                req.settlementDate(),
                new MonetaryValue(req.priceCurrency(), req.pricePerUnit()),
                new MonetaryValue(req.premiumCurrency(), req.premium()),
                buyer,
                seller,
                req.amount(),
                modifier
        );

        int partnerRouting = (entity.getBuyerRoutingNumber() == myRouting)
                ? entity.getSellerRoutingNumber()
                : entity.getBuyerRoutingNumber();
        ForeignBankId remoteRef = remoteForeignBankIdOf(entity);

        ResponseEntity<Void> partnerResp = interbankClient.outboundPutCounter(
                partnerRouting, remoteRef, offer);

        if (partnerResp.getStatusCode().is2xxSuccessful()) {
            entity.setAmount(req.amount());
            entity.setPriceCurrency(req.priceCurrency().name());
            entity.setPriceAmount(req.pricePerUnit());
            entity.setPremiumCurrency(req.premiumCurrency().name());
            entity.setPremiumAmount(req.premium());
            entity.setSettlementDate(req.settlementDate());
            entity.setLastModifiedByRouting(myRouting);
            entity.setLastModifiedById(myUserIdStr);
            negRepo.save(entity);
            log.info("Counter-offer outbound negotiation {} accepted by partner {}",
                    id, partnerRouting);
            return ResponseEntity.status(partnerResp.getStatusCode()).body(
                    new OutboundNegotiationResponse(entity.getId(), remoteRef, otcService.toDto(entity)));
        } else {
            log.warn("Counter-offer outbound negotiation {} partner returned {}",
                    id, partnerResp.getStatusCode());
            return ResponseEntity.status(partnerResp.getStatusCode()).build();
        }
    }

    /**
     * POST /api/interbank/otc/negotiations/{id}/accept — accept current offer.
     *
     * <p>Mi smo buyer-bank, saljemo /accept ka seller-bank-i. Partner pokrece
     * 2PC kao coordinator: posalje nam NEW_TX → mi prepareLocal → COMMIT_TX → mi
     * commitLocal. Kad nas {@link com.banka1.interbank.controller.InterbankInboundController}
     * obradi COMMIT_TX, partner-ov accept endpoint vraca 204 nama.
     *
     * <p>Lokalno cleanup: posle uspesnog 204 od partner-a, lokalni mirror je vec
     * azuriran kroz inbound dispatch (negotiation flip is_ongoing=false +
     * contract create iz {@link com.banka1.interbank.service.InterbankCoordinatorService}
     * sequence). Mi samo trigerujemo flow.
     */
    @Transactional
    public ResponseEntity<Void> acceptOutbound(String id) {
        int myRouting = props.getMyRoutingNumber();
        InterbankNegotiationEntity entity = findByLocalOrRemoteId(id);

        if (!entity.isOngoing()) {
            throw new NegotiationClosedException("Negotiation " + id + " is closed");
        }
        if (entity.getSettlementDate() != null
                && entity.getSettlementDate().isBefore(OffsetDateTime.now())) {
            throw new NegotiationClosedException("Negotiation " + id + " settlement date passed");
        }
        // Turn check: mi mozemo accept SAMO ako je partner zadnji modifikovao.
        if (entity.getLastModifiedByRouting() == myRouting) {
            throw new TurnViolationException(
                    "Not your turn to accept — your bank made the last modification");
        }
        // Validacija: mi MORAMO biti buyer u outbound accept-u (buyer-bank uvek
        // accept-uje ponudu seller-a).
        if (entity.getBuyerRoutingNumber() != myRouting) {
            throw new InvalidNegotiationException(
                    "Cannot accept — we are not buyer-bank for this negotiation");
        }

        int partnerRouting = entity.getSellerRoutingNumber();
        ForeignBankId remoteRef = remoteForeignBankIdOf(entity);

        ResponseEntity<Void> partnerResp = interbankClient.outboundAccept(partnerRouting, remoteRef);
        log.info("Accept outbound negotiation {} partner returned {}", id, partnerResp.getStatusCode());
        return partnerResp;
    }

    /**
     * DELETE /api/interbank/otc/negotiations/{id} — close pregovor.
     *
     * <p>DELETE ka partner-u + flip lokalni mirror. Idempotent.
     */
    @Transactional
    public ResponseEntity<Void> deleteOutbound(String id) {
        int myRouting = props.getMyRoutingNumber();
        InterbankNegotiationEntity entity = findByLocalOrRemoteId(id);

        int partnerRouting = (entity.getBuyerRoutingNumber() == myRouting)
                ? entity.getSellerRoutingNumber()
                : entity.getBuyerRoutingNumber();
        ForeignBankId remoteRef = remoteForeignBankIdOf(entity);

        ResponseEntity<Void> partnerResp = interbankClient.outboundDelete(partnerRouting, remoteRef);
        if (partnerResp.getStatusCode().is2xxSuccessful() && entity.isOngoing()) {
            entity.setOngoing(false);
            negRepo.save(entity);
            log.info("Deleted outbound negotiation {}", id);
        }
        return partnerResp;
    }

    /**
     * Helper: lookup po local id-u, fallback na remote id u entity-jima koji
     * nisu authoritative.
     */
    InterbankNegotiationEntity findByLocalOrRemoteId(String id) {
        return negRepo.findById(id)
                .or(() -> negRepo.findAll().stream()
                        .filter(e -> id.equals(e.getRemoteNegotiationId()))
                        .findFirst())
                .orElseThrow(() -> new NegotiationNotFoundException(
                        "Negotiation " + id + " not found"));
    }

    /**
     * PR_33 follow-up: GET /api/interbank/otc/public-stock?bankCode=...
     * Discovery: lista javnih akcija iz partner banke (npr. Banka 2).
     * Frontend pravi tabelu pa user moze direktno da krene "Napravi ponudu".
     */
    @Transactional(readOnly = true)
    public java.util.List<com.banka1.interbank.otc.dto.PublicStockEntryDto> fetchPartnerPublicStock(int bankCode) {
        if (bankCode == props.getMyRoutingNumber()) {
            // Beskorisno pitati nas sami sebe kroz outbound; vratimo prazno.
            return java.util.List.of();
        }
        return interbankClient.outboundFetchPublicStock(bankCode);
    }

    /**
     * Helper: gradi {@link ForeignBankId} koji frontend koristi kao "counterparty
     * bank reference" za badge ("Banka 2" vs "Naša banka") i za inter-bank
     * routing iz UI-a. Uvek vraca PARTNER bank routing — onaj koji NIJE nas —
     * nezavisno od {@code isAuthoritative} flag-a (FE ne treba da zna gde live
     * authoritative kopija).
     *
     * <p>Ako smo authoritative (mi pokrenuli pregovor), remote id je nas
     * lokalni neg-id; partner ce ga prepoznati. Ako nismo, koristimo
     * {@code remoteNegotiationId} (partner-ov id) ako je popunjen.
     */
    private ForeignBankId remoteForeignBankIdOf(InterbankNegotiationEntity e) {
        int myRouting = props.getMyRoutingNumber();
        int partnerRouting = (e.getBuyerRoutingNumber() == myRouting)
                ? e.getSellerRoutingNumber()
                : e.getBuyerRoutingNumber();
        String remoteId = e.getRemoteNegotiationId() != null
                ? e.getRemoteNegotiationId()
                : e.getId();
        return new ForeignBankId(partnerRouting, remoteId);
    }

    /**
     * Generise nas lokalni mirror id (paritet sa {@link OtcNegotiationService}).
     */
    private String generateNegotiationId() {
        byte[] bytes = new byte[8];
        random.nextBytes(bytes);
        return "neg-" + HexFormat.of().formatHex(bytes);
    }

    /** Visible-for-tests helper. */
    static CurrencyCode parseCurrency(String s) {
        return CurrencyCode.valueOf(s);
    }
}
