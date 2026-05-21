package com.banka1.order.entity;

import com.banka1.order.entity.enums.OtcNegotiationEventType;
import com.banka1.order.entity.enums.OtcNegotiationStatus;
import jakarta.persistence.*;
import lombok.Getter;
import lombok.NoArgsConstructor;
import lombok.Setter;

import java.math.BigDecimal;
import java.time.LocalDate;
import java.time.LocalDateTime;

/**
 * Append-only history row describing one OTC negotiation mutation.
 */
@Entity
@Table(name = "otc_negotiation_history")
@Getter
@Setter
@NoArgsConstructor
public class OtcNegotiationHistory {

    @Id
    @GeneratedValue(strategy = GenerationType.IDENTITY)
    private Long id;

    @Column(name = "negotiation_id", nullable = false)
    private Long negotiationId;

    @Column(name = "actor_user_id", nullable = false)
    private Long actorUserId;

    @Enumerated(EnumType.STRING)
    @Column(name = "event_type", nullable = false)
    private OtcNegotiationEventType eventType;

    @Column(name = "previous_quantity")
    private Integer previousQuantity;

    @Column(name = "new_quantity")
    private Integer newQuantity;

    @Column(name = "previous_price_per_unit", precision = 19, scale = 4)
    private BigDecimal previousPricePerUnit;

    @Column(name = "new_price_per_unit", precision = 19, scale = 4)
    private BigDecimal newPricePerUnit;

    @Column(name = "previous_contract_expiry_date")
    private LocalDate previousContractExpiryDate;

    @Column(name = "new_contract_expiry_date")
    private LocalDate newContractExpiryDate;

    @Enumerated(EnumType.STRING)
    @Column(name = "previous_status")
    private OtcNegotiationStatus previousStatus;

    @Enumerated(EnumType.STRING)
    @Column(name = "resulting_status", nullable = false)
    private OtcNegotiationStatus resultingStatus;

    @Column(name = "changed_at", nullable = false)
    private LocalDateTime changedAt;

    @PrePersist
    void onCreate() {
        changedAt = LocalDateTime.now();
    }
}
