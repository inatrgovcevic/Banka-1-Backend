package com.banka1.tradingservice.otc.dto;

import jakarta.validation.constraints.DecimalMin;
import jakarta.validation.constraints.Future;
import jakarta.validation.constraints.NotBlank;
import jakarta.validation.constraints.NotNull;
import lombok.AllArgsConstructor;
import lombok.Data;
import lombok.NoArgsConstructor;

import java.math.BigDecimal;
import java.time.LocalDate;

/**
 * Inicijalna OTC ponuda od kupca prodavcu. Posle ovoga ponuda ima status PENDING_SELLER.
 */
@Data
@NoArgsConstructor
@AllArgsConstructor
public class CreateOtcOfferRequest {

    @NotBlank
    private String stockTicker;

    @NotNull
    private Long sellerId;

    @NotNull
    @DecimalMin(value = "1")
    private Integer amount;

    @NotNull
    @DecimalMin(value = "0.00", inclusive = false)
    private BigDecimal pricePerStock;

    @NotNull
    @DecimalMin(value = "0.00")
    private BigDecimal premium;

    @NotNull
    @Future
    private LocalDate settlementDate;
}
