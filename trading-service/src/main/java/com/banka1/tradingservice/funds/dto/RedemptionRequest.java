package com.banka1.tradingservice.funds.dto;

import jakarta.validation.constraints.DecimalMin;
import jakarta.validation.constraints.NotBlank;
import jakarta.validation.constraints.NotNull;
import lombok.AllArgsConstructor;
import lombok.Data;
import lombok.NoArgsConstructor;

import java.math.BigDecimal;

/** Klijent povlaci sredstva — body za POST /funds/{id}/redeem. */
@Data
@NoArgsConstructor
@AllArgsConstructor
public class RedemptionRequest {

    @NotNull
    @DecimalMin(value = "0.01")
    private BigDecimal amount;

    /** Klijentov tekuci racun na koji ide isplata. */
    @NotBlank
    private String toAccountNumber;
}
