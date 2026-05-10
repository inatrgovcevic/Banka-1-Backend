package com.banka1.banking_service.account_service.dto.request;

import jakarta.validation.constraints.DecimalMin;
import jakarta.validation.constraints.NotNull;
import jakarta.validation.constraints.Pattern;
import lombok.AllArgsConstructor;
import lombok.Getter;
import lombok.NoArgsConstructor;
import lombok.Setter;

import java.math.BigDecimal;

@Getter
@Setter
@NoArgsConstructor
@AllArgsConstructor
public class CreditDebitAccountDto {
    @Pattern(regexp = "^\\d{19}$", message = "Broj racuna mora imati 19 cifara")
    private String accountNumber;
    @NotNull(message = "Unesi amount")
    @DecimalMin(value = "0.0", inclusive = false)
    private BigDecimal amount;
    @NotNull(message = "Unesi id clienta")
    private Long clientId;
}
