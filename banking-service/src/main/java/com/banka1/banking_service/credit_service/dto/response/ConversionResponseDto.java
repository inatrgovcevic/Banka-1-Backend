package com.banka1.banking_service.credit_service.dto.response;

import java.math.BigDecimal;
import java.time.LocalDate;

/**
 * DTO response for currency conversion calculation endpoint.
 * The structure is tailored to the task specification and represents the public API
 * contract that clients receive as a JSON response.
 *
 * @param fromCurrency the source currency code
 * @param toCurrency the target currency code
 * @param fromAmount the original amount from the request
 * @param toAmount the converted amount in the target currency
 * @param rate the effective conversion rate, i.e., the ratio of {@code toAmount/fromAmount}
 * @param commission the calculated commission in the source currency
 * @param date the exchange rate date used in the calculation
 */

public record ConversionResponseDto(
        String fromCurrency,
        String toCurrency,
        BigDecimal fromAmount,
        BigDecimal toAmount,
        BigDecimal rate,
        BigDecimal commission,
        LocalDate date
) {
}
