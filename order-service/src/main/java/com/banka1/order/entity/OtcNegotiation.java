package com.banka1.order.entity;

import com.banka1.order.entity.enums.OtcNegotiationStatus;
import jakarta.persistence.*;
import lombok.Getter;
import lombok.NoArgsConstructor;
import lombok.Setter;

import java.math.BigDecimal;
import java.time.LocalDate;
import java.time.LocalDateTime;

/**
 * Current snapshot of one OTC negotiation between a buyer and seller.
 */
@Entity
@Table(name = "otc_negotiation")
@Getter
@Setter
@NoArgsConstructor
public class OtcNegotiation {

    @Id
    @GeneratedValue(strategy = GenerationType.IDENTITY)
    private Long id;

    @Column(name = "buyer_id", nullable = false)
    private Long buyerId;

    @Column(name = "seller_id", nullable = false)
    private Long sellerId;

    @Column(name = "seller_portfolio_id", nullable = false)
    private Long sellerPortfolioId;

    @Column(name = "listing_id", nullable = false)
    private Long listingId;

    @Column(nullable = false)
    private Integer quantity;

    @Column(name = "price_per_unit", nullable = false, precision = 19, scale = 4)
    private BigDecimal pricePerUnit;

    @Column(name = "contract_expiry_date", nullable = false)
    private LocalDate contractExpiryDate;

    @Enumerated(EnumType.STRING)
    @Column(nullable = false)
    private OtcNegotiationStatus status;

    @Column(name = "created_by_user_id", nullable = false)
    private Long createdByUserId;

    @Column(name = "last_updated_by_user_id", nullable = false)
    private Long lastUpdatedByUserId;

    @Column(name = "created_at", nullable = false)
    private LocalDateTime createdAt;

    @Column(name = "updated_at", nullable = false)
    private LocalDateTime updatedAt;

    @Column(name = "expiration_notified_at")
    private LocalDate expirationNotifiedAt;

    @PrePersist
    void onCreate() {
        LocalDateTime now = LocalDateTime.now();
        createdAt = now;
        updatedAt = now;
    }

    @PreUpdate
    void onUpdate() {
        updatedAt = LocalDateTime.now();
    }
}
