package com.banka1.interbank.model;

import jakarta.persistence.Column;
import jakarta.persistence.Entity;
import jakarta.persistence.Id;
import jakarta.persistence.PrePersist;
import jakarta.persistence.PreUpdate;
import jakarta.persistence.Table;
import jakarta.persistence.Version;
import java.math.BigDecimal;
import java.time.Instant;
import java.time.OffsetDateTime;
import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Getter;
import lombok.NoArgsConstructor;
import lombok.Setter;

/**
 * PR_32 Phase 3: OTC pregovor izmedju dve banke (Tim 2 §3 negotiation
 * protokol).
 *
 * <p>{@code isAuthoritative} ozanacava da li smo mi seller (vlasnik state-a)
 * ili samo mirror koji prima counter-offer poruke. {@code remoteNegotiationId}
 * se popunjava samo kad nismo authoritative — drzimo partner-ov ID za
 * cross-referencing.
 *
 * <p>{@code @Version} (D14 iz spec-a) sprecava concurrent counter-offer
 * write-ove kroz optimistic locking.
 */
@Entity
@Table(name = "interbank_negotiations")
@Getter
@Setter
@NoArgsConstructor
@AllArgsConstructor
@Builder
public class InterbankNegotiationEntity {

    @Id
    @Column(length = 64)
    private String id;

    @Column(name = "buyer_routing_number", nullable = false)
    private int buyerRoutingNumber;

    @Column(name = "buyer_id", nullable = false, length = 64)
    private String buyerId;

    @Column(name = "seller_routing_number", nullable = false)
    private int sellerRoutingNumber;

    @Column(name = "seller_id", nullable = false, length = 64)
    private String sellerId;

    @Column(name = "stock_ticker", nullable = false, length = 16)
    private String stockTicker;

    @Column(nullable = false)
    private int amount;

    @Column(name = "price_currency", nullable = false, length = 8)
    private String priceCurrency;

    @Column(name = "price_amount", nullable = false, precision = 20, scale = 4)
    private BigDecimal priceAmount;

    @Column(name = "premium_currency", nullable = false, length = 8)
    private String premiumCurrency;

    @Column(name = "premium_amount", nullable = false, precision = 20, scale = 4)
    private BigDecimal premiumAmount;

    @Column(name = "settlement_date", nullable = false)
    private OffsetDateTime settlementDate;

    @Column(name = "last_modified_by_routing", nullable = false)
    private int lastModifiedByRouting;

    @Column(name = "last_modified_by_id", nullable = false, length = 64)
    private String lastModifiedById;

    @Column(name = "is_ongoing", nullable = false)
    private boolean isOngoing;

    @Column(name = "is_authoritative", nullable = false)
    private boolean isAuthoritative;

    @Column(name = "remote_negotiation_id", length = 64)
    private String remoteNegotiationId;

    @Column(name = "linked_local_offer_id")
    private Long linkedLocalOfferId;

    @Version
    @Column(nullable = false)
    private Long version;

    @Column(name = "created_at", nullable = false, updatable = false)
    private Instant createdAt;

    @Column(name = "last_modified_at", nullable = false)
    private Instant lastModifiedAt;

    @PrePersist
    void onCreate() {
        Instant now = Instant.now();
        if (createdAt == null) {
            createdAt = now;
        }
        if (lastModifiedAt == null) {
            lastModifiedAt = now;
        }
    }

    @PreUpdate
    void onUpdate() {
        lastModifiedAt = Instant.now();
    }
}
