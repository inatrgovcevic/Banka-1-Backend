package com.banka1.interbank.service;

import com.banka1.interbank.config.InterbankProperties;
import com.banka1.interbank.exception.InvalidNegotiationException;
import com.banka1.interbank.exception.NegotiationClosedException;
import com.banka1.interbank.exception.NegotiationNotFoundException;
import com.banka1.interbank.exception.TurnViolationException;
import com.banka1.interbank.model.InterbankNegotiationEntity;
import com.banka1.interbank.otc.dto.OtcNegotiationDto;
import com.banka1.interbank.otc.dto.OtcOfferDto;
import com.banka1.interbank.protocol.dto.CurrencyCode;
import com.banka1.interbank.protocol.dto.ForeignBankId;
import com.banka1.interbank.protocol.dto.MonetaryValue;
import com.banka1.interbank.protocol.dto.StockDescription;
import com.banka1.interbank.repository.InterbankNegotiationRepository;
import java.math.BigDecimal;
import java.security.SecureRandom;
import java.time.OffsetDateTime;
import java.util.HexFormat;
import lombok.RequiredArgsConstructor;
import lombok.extern.slf4j.Slf4j;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;

/**
 * PR_32 Phase 10 Task 10.3: OTC §3 CRUD + turn-logic service.
 *
 * <p>Implementira sledece Tim 2 spec §3 rute:
 * <ul>
 *   <li>§3.2 POST /negotiations → {@link #createNegotiation}</li>
 *   <li>§3.3 PUT /negotiations/{rn}/{id} → {@link #updateCounter}</li>
 *   <li>§3.4 GET /negotiations/{rn}/{id} → {@link #getNegotiation}</li>
 *   <li>§3.5 DELETE /negotiations/{rn}/{id} → {@link #delete}</li>
 *   <li>§3.6 GET /negotiations/{rn}/{id}/accept → {@link #acceptNegotiation}
 *       (delegira na {@link InterbankCoordinatorService} 2PC orchestrator)</li>
 * </ul>
 *
 * <p><strong>KRITICNO — Tim 2 §6.3 update:</strong> turn violation i
 * closed-negotiation slucajevi vracaju 409, NE 400.
 * {@link InvalidNegotiationException} = 400 se koristi samo za malformed
 * payload (settlement u proslosti, routing mismatch, negativan iznos).
 */
@Service
@RequiredArgsConstructor
@Slf4j
public class OtcNegotiationService {

    private final InterbankNegotiationRepository negRepo;
    private final InterbankProperties props;
    private final InterbankCoordinatorService coordinator;
    private final SecureRandom random = new SecureRandom();

    /**
     * §3.2 POST /negotiations — buyer iz druge banke kreira novi pregovor.
     * Validira da smo MI seller bank (sellerId.routingNumber == myRouting) i
     * da je buyerId.routingNumber == X-Api-Key sender. Generisemo lokalni
     * negotiation ID, persistujemo entity sa is_authoritative=true,
     * is_ongoing=true.
     *
     * @return {@link ForeignBankId} sa nasim routing brojem i generisanim
     *         negotiation ID-jem
     */
    @Transactional
    public ForeignBankId createNegotiation(OtcOfferDto offer, int senderRoutingNumber) {
        int myRouting = props.getMyRoutingNumber();
        if (offer.sellerId().routingNumber() != myRouting) {
            throw new InvalidNegotiationException(
                    "sellerId.routingNumber must be " + myRouting + " (this bank)");
        }
        if (offer.buyerId().routingNumber() != senderRoutingNumber) {
            throw new InvalidNegotiationException(
                    "buyerId.routingNumber must match X-Api-Key sender (" + senderRoutingNumber + ")");
        }
        if (offer.lastModifiedBy().routingNumber() != senderRoutingNumber) {
            throw new InvalidNegotiationException(
                    "lastModifiedBy.routingNumber must match X-Api-Key sender (" + senderRoutingNumber + ")");
        }
        if (offer.settlementDate().isBefore(OffsetDateTime.now())) {
            throw new InvalidNegotiationException("settlementDate must be in the future");
        }
        if (offer.amount() <= 0) {
            throw new InvalidNegotiationException("amount must be positive");
        }
        if (offer.pricePerUnit().amount() == null
                || offer.pricePerUnit().amount().signum() <= 0) {
            throw new InvalidNegotiationException("pricePerUnit.amount must be positive");
        }
        if (offer.premium().amount() == null
                || offer.premium().amount().signum() < 0) {
            throw new InvalidNegotiationException("premium.amount must be non-negative");
        }

        String id = generateNegotiationId();
        InterbankNegotiationEntity entity = InterbankNegotiationEntity.builder()
                .id(id)
                .buyerRoutingNumber(offer.buyerId().routingNumber())
                .buyerId(offer.buyerId().id())
                .sellerRoutingNumber(offer.sellerId().routingNumber())
                .sellerId(offer.sellerId().id())
                .stockTicker(offer.stock().ticker())
                .amount(offer.amount())
                .priceCurrency(offer.pricePerUnit().currency().name())
                .priceAmount(offer.pricePerUnit().amount())
                .premiumCurrency(offer.premium().currency().name())
                .premiumAmount(offer.premium().amount())
                .settlementDate(offer.settlementDate())
                .lastModifiedByRouting(offer.lastModifiedBy().routingNumber())
                .lastModifiedById(offer.lastModifiedBy().id())
                .isOngoing(true)
                .isAuthoritative(true)
                .build();
        negRepo.save(entity);
        log.info("Created interbank negotiation {} (buyer={}/{}, seller={}/{})",
                id, offer.buyerId().routingNumber(), offer.buyerId().id(),
                offer.sellerId().routingNumber(), offer.sellerId().id());
        return new ForeignBankId(myRouting, id);
    }

    /**
     * §3.4 GET /negotiations/{rn}/{id} — vraca trenutno stanje. Per Tim 2 §3.4,
     * {@code rn} u path-u je routing broj banke koja je gospodar (authoritative)
     * pregovora. U nasoj implementaciji svi negotiationi sa is_authoritative=true
     * imaju rn = myRouting, ali ne fail-ujemo ako se rn ne poklapa — samo
     * vracamo ono sto imamo (mirror state za partner banke).
     */
    @Transactional(readOnly = true)
    public OtcNegotiationDto getNegotiation(int rn, String id) {
        InterbankNegotiationEntity entity = negRepo.findById(id)
                .orElseThrow(() -> new NegotiationNotFoundException(
                        "Negotiation " + rn + "/" + id + " not found"));
        return toDto(entity);
    }

    /**
     * §3.3 PUT /negotiations/{rn}/{id} — counter-offer.
     *
     * <p><strong>KRITICNO Tim 2 §6.3:</strong>
     * <ul>
     *   <li>204 No Content — happy path</li>
     *   <li>409 Conflict — turn violation (lastModifiedBy == prethodni
     *       modifier) ILI negotiation zatvoren</li>
     *   <li>400 Bad Request — malformed payload (routing mismatch, past
     *       settlement, etc.)</li>
     *   <li>404 Not Found — negotiation ne postoji</li>
     * </ul>
     */
    @Transactional
    public void updateCounter(int rn, String id, OtcOfferDto offer, int senderRoutingNumber) {
        InterbankNegotiationEntity entity = negRepo.findById(id)
                .orElseThrow(() -> new NegotiationNotFoundException(
                        "Negotiation " + id + " not found"));

        if (!entity.isOngoing()) {
            throw new NegotiationClosedException("Negotiation " + id + " is closed");
        }
        if (entity.getSettlementDate() != null
                && entity.getSettlementDate().isBefore(OffsetDateTime.now())) {
            throw new NegotiationClosedException(
                    "Negotiation " + id + " settlement date passed");
        }

        // lastModifiedBy.routingNumber MORA biti X-Api-Key sender
        if (offer.lastModifiedBy().routingNumber() != senderRoutingNumber) {
            throw new InvalidNegotiationException(
                    "lastModifiedBy.routingNumber must match X-Api-Key sender");
        }

        // Turn check (KRITICNO — 409 ne 400):
        // ako je trenutni entity.lastModifiedBy potice iz sender-ove banke,
        // sender je upravo modifikovao i ne sme opet — red je na drugoj strani.
        if (entity.getLastModifiedByRouting() == senderRoutingNumber) {
            throw new TurnViolationException(
                    "Not your turn to modify — last modification was from your bank");
        }

        // Validacije payload-a
        if (offer.settlementDate().isBefore(OffsetDateTime.now())) {
            throw new InvalidNegotiationException("settlementDate must be in the future");
        }
        if (offer.amount() <= 0) {
            throw new InvalidNegotiationException("amount must be positive");
        }
        if (offer.pricePerUnit().amount() == null
                || offer.pricePerUnit().amount().signum() <= 0) {
            throw new InvalidNegotiationException("pricePerUnit.amount must be positive");
        }
        if (offer.premium().amount() == null
                || offer.premium().amount().signum() < 0) {
            throw new InvalidNegotiationException("premium.amount must be non-negative");
        }

        entity.setAmount(offer.amount());
        entity.setPriceAmount(offer.pricePerUnit().amount());
        entity.setPriceCurrency(offer.pricePerUnit().currency().name());
        entity.setPremiumAmount(offer.premium().amount());
        entity.setPremiumCurrency(offer.premium().currency().name());
        entity.setSettlementDate(offer.settlementDate());
        entity.setLastModifiedByRouting(offer.lastModifiedBy().routingNumber());
        entity.setLastModifiedById(offer.lastModifiedBy().id());
        negRepo.save(entity);
        log.info("Updated negotiation {} (counter-offer by {}/{})",
                id, offer.lastModifiedBy().routingNumber(), offer.lastModifiedBy().id());
    }

    /**
     * §3.5 DELETE /negotiations/{rn}/{id} — close negotiation.
     * Idempotent: ako je vec zatvoren, ne baca exception (samo no-op flip).
     */
    @Transactional
    public void delete(int rn, String id) {
        InterbankNegotiationEntity entity = negRepo.findById(id)
                .orElseThrow(() -> new NegotiationNotFoundException(
                        "Negotiation " + id + " not found"));
        if (entity.isOngoing()) {
            entity.setOngoing(false);
            negRepo.save(entity);
            log.info("Closed negotiation {}", id);
        }
    }

    /**
     * §3.6 GET /negotiations/{rn}/{id}/accept — buyer iz druge banke prihvata
     * trenutnu ponudu. Delegira na {@link InterbankCoordinatorService} koji
     * sinhroni 2PC kroz partner banku. Per Tim 2 §6.6, konekcija moze drzati
     * do 60s dok 2PC ne commit-uje.
     */
    public void acceptNegotiation(int rn, String id, int senderRoutingNumber) {
        InterbankNegotiationEntity entity = negRepo.findById(id)
                .orElseThrow(() -> new NegotiationNotFoundException(
                        "Negotiation " + id + " not found"));
        if (!entity.isOngoing()) {
            throw new NegotiationClosedException("Negotiation " + id + " is closed");
        }
        if (entity.getSettlementDate() != null
                && entity.getSettlementDate().isBefore(OffsetDateTime.now())) {
            throw new NegotiationClosedException(
                    "Negotiation " + id + " settlement date passed");
        }
        // Turn check za accept: prihvatilac ne sme da bude poslednji modifier.
        if (entity.getLastModifiedByRouting() == senderRoutingNumber) {
            throw new TurnViolationException(
                    "Not your turn to accept — your bank made the last modification");
        }
        coordinator.acceptNegotiation(entity);
    }

    /**
     * Konvertuj JPA entity u protokol DTO. Public za reuse iz controller-a u
     * inline transformaciji.
     */
    public OtcNegotiationDto toDto(InterbankNegotiationEntity e) {
        return new OtcNegotiationDto(
                new StockDescription(e.getStockTicker()),
                e.getSettlementDate(),
                new MonetaryValue(CurrencyCode.valueOf(e.getPriceCurrency()), e.getPriceAmount()),
                new MonetaryValue(CurrencyCode.valueOf(e.getPremiumCurrency()), e.getPremiumAmount()),
                new ForeignBankId(e.getBuyerRoutingNumber(), e.getBuyerId()),
                new ForeignBankId(e.getSellerRoutingNumber(), e.getSellerId()),
                e.getAmount(),
                new ForeignBankId(e.getLastModifiedByRouting(), e.getLastModifiedById()),
                e.isOngoing()
        );
    }

    private String generateNegotiationId() {
        byte[] bytes = new byte[8];
        random.nextBytes(bytes);
        return "neg-" + HexFormat.of().formatHex(bytes);
    }

    /** Visibility helper used by tests to seed BigDecimal valori. */
    static BigDecimal bd(String v) {
        return new BigDecimal(v);
    }
}
