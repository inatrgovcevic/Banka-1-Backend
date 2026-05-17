package com.banka1.order.dto.client;

import lombok.AllArgsConstructor;
import lombok.Data;
import lombok.NoArgsConstructor;

import java.math.BigDecimal;

/**
 * Order-service mirror DTO-a iz account-service-a za one-sided debit/credit
 * trade-leg operaciju (GHI #199). Po PM direktivi, trade leg klijentskog BUY/SELL
 * ne sme da prolazi kroz bankin racun - ovaj DTO se posle salje na
 * {@code /internal/accounts/exchange/buy} (debit) ili {@code .../exchange/sell}
 * (credit).
 */
@Data
@NoArgsConstructor
@AllArgsConstructor
public class OneSidedTransactionDto {
    private String accountNumber;
    private Long accountId;
    private BigDecimal amount;
    private Long clientId;
    private String description;
}
