package com.banka1.order.dto;

import com.banka1.order.entity.enums.OtcNegotiationStatus;
import lombok.Data;

import java.math.BigDecimal;
import java.time.LocalDate;
import java.time.LocalDateTime;

/**
 * Current OTC negotiation state returned by the API.
 */
@Data
public class OtcNegotiationResponse {
    private Long id;
    private Long buyerId;
    private Long sellerId;
    private Long sellerPortfolioId;
    private Long listingId;
    private Integer quantity;
    private BigDecimal pricePerUnit;
    private LocalDate contractExpiryDate;
    private OtcNegotiationStatus status;
    private Long createdByUserId;
    private Long lastUpdatedByUserId;
    private Long counterpartyId;
    private LocalDateTime createdAt;
    private LocalDateTime updatedAt;
}
