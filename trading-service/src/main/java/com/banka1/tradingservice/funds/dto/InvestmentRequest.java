package com.banka1.tradingservice.funds.dto;

import jakarta.validation.constraints.DecimalMin;
import jakarta.validation.constraints.NotBlank;
import jakarta.validation.constraints.NotNull;
import lombok.AllArgsConstructor;
import lombok.Data;
import lombok.NoArgsConstructor;

import java.math.BigDecimal;

/** Klijent uplatuje u fond — body za POST /funds/{id}/invest. */
@Data
@NoArgsConstructor
@AllArgsConstructor
public class InvestmentRequest {

    @NotNull
    @DecimalMin(value = "0.01")
    private BigDecimal amount;

    /** Klijentov tekuci racun sa kojeg ide uplata. */
    @NotBlank
    private String fromAccountNumber;
}
