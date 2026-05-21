package com.banka1.order.dto;

import jakarta.validation.constraints.Future;
import jakarta.validation.constraints.Min;
import jakarta.validation.constraints.NotNull;
import lombok.Data;

import java.math.BigDecimal;
import java.time.LocalDate;

/**
 * Request for creating a new OTC negotiation against a public seller portfolio.
 */
@Data
public class CreateOtcNegotiationRequest {

    @NotNull
    private Long sellerPortfolioId;

    @NotNull
    @Min(1)
    private Integer quantity;

    @NotNull
    private BigDecimal pricePerUnit;

    @NotNull
    @Future
    private LocalDate contractExpiryDate;
}
