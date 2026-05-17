package com.banka1.interbank.model;

import com.banka1.interbank.model.enums.NegotiationContractStatus;
import jakarta.persistence.Column;
import jakarta.persistence.Entity;
import jakarta.persistence.EnumType;
import jakarta.persistence.Enumerated;
import jakarta.persistence.Id;
import jakarta.persistence.PrePersist;
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
 * PR_32 Phase 3: finalizovani interbank opcioni ugovor.
 *
 * <p>{@code negotiationId} referencira {@link InterbankNegotiationEntity#getId()}
 * (FK constraint vec u Liquibase schemi). {@code status} pocinje od
 * {@code PENDING_PREMIUM} dok premium 2PC ne commit-uje, posle cega prelazi u
 * {@code ACTIVE} — ovo je KRIT bug #2 fix iz CLAUDE.md sekcije 4 (option
 * contract ne sme biti ACTIVE pre nego sto premium stigne na seller-a).
 */
@Entity
@Table(name = "interbank_contracts")
@Getter
@Setter
@NoArgsConstructor
@AllArgsConstructor
@Builder
public class InterbankContractEntity {

    @Id
    @Column(length = 64)
    private String id;

    @Column(name = "negotiation_id", nullable = false, length = 64)
    private String negotiationId;

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

    @Column(name = "strike_currency", nullable = false, length = 8)
    private String strikeCurrency;

    @Column(name = "strike_amount", nullable = false, precision = 20, scale = 4)
    private BigDecimal strikeAmount;

    @Column(name = "settlement_date", nullable = false)
    private OffsetDateTime settlementDate;

    @Enumerated(EnumType.STRING)
    @Column(nullable = false, length = 32)
    private NegotiationContractStatus status;

    @Column(name = "option_pseudo_owner_routing", nullable = false)
    private int optionPseudoOwnerRouting;

    @Column(name = "option_pseudo_owner_id", nullable = false, length = 64)
    private String optionPseudoOwnerId;

    @Version
    @Column(nullable = false)
    private Long version;

    @Column(name = "created_at", nullable = false, updatable = false)
    private Instant createdAt;

    @Column(name = "exercised_at")
    private Instant exercisedAt;

    @Column(name = "expired_at")
    private Instant expiredAt;

    @PrePersist
    void onCreate() {
        if (createdAt == null) {
            createdAt = Instant.now();
        }
    }
}
