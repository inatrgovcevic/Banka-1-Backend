package com.banka1.banking_service.account_service.dto.request;

import jakarta.validation.constraints.DecimalMin;
import jakarta.validation.constraints.NotBlank;
import jakarta.validation.constraints.NotNull;
import lombok.AllArgsConstructor;
import lombok.Getter;
import lombok.NoArgsConstructor;
import lombok.Setter;

import java.math.BigDecimal;

@Getter
@Setter
@NoArgsConstructor
@AllArgsConstructor
public class CreditDebitBankDto {
    @NotBlank(message = "Unesi currencyCode")
    private String currencyCode;
    @NotNull(message = "Unesi amount")
    @DecimalMin(value = "0.0", inclusive = false)
    private BigDecimal amount;
}