package com.banka1.order.dto;

import com.banka1.order.entity.enums.OtcNegotiationEventType;
import com.banka1.order.entity.enums.OtcNegotiationStatus;
import lombok.Data;

import java.math.BigDecimal;
import java.time.LocalDate;
import java.time.LocalDateTime;

/**
 * One append-only OTC negotiation history row.
 */
@Data
public class OtcNegotiationHistoryResponse {
    private Long id;
    private Long negotiationId;
    private Long actorUserId;
    private OtcNegotiationEventType eventType;
    private Integer previousQuantity;
    private Integer newQuantity;
    private BigDecimal previousPricePerUnit;
    private BigDecimal newPricePerUnit;
    private LocalDate previousContractExpiryDate;
    private LocalDate newContractExpiryDate;
    private OtcNegotiationStatus previousStatus;
    private OtcNegotiationStatus resultingStatus;
    private LocalDateTime changedAt;
}
