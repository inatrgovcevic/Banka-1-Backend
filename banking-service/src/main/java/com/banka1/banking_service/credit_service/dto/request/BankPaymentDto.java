package com.banka1.banking_service.credit_service.dto.request;

import lombok.AllArgsConstructor;
import lombok.Getter;
import lombok.NoArgsConstructor;
import lombok.Setter;

import java.math.BigDecimal;

@AllArgsConstructor
@NoArgsConstructor
@Getter
@Setter
public class BankPaymentDto {
    /**
     * Broj računa na koji se novac prenosi (19 cifara).
     */

    private String fromAccountNumber;
    private String toAccountNumber;
    private BigDecimal amount;


}
