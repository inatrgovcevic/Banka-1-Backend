package com.banka1.banking_service.credit_service.dto.request;

import lombok.Getter;
import lombok.Setter;

import java.math.BigDecimal;
import java.time.LocalDate;

/**
 * DTO that maps query parameters from currency conversion endpoint.
 * Spring MVC automatically binds values from URL query string into this object
 * and then applies Bean Validation annotations defined on the fields.
 * Used for GET /calculate?fromCurrency=...&toCurrency=...&amount=... endpoint.
 */
@Getter
@Setter
public class ConversionQueryDto {

    private static final String SUPPORTED_CURRENCY_REGEX = "^(?i)(RSD|EUR|CHF|USD|GBP|JPY|CAD|AUD)$";
    private static final String SUPPORTED_CURRENCY_MESSAGE =
            "Supported currencies are RSD, EUR, CHF, USD, GBP, JPY, CAD and AUD.";

    /**
     * The source currency from which the amount is being converted.
     */
    private String fromCurrency;

    /**
     * The target currency to which the amount is being converted.
     */
    private String toCurrency;

    /**
     * The amount to be converted.
     */
    private BigDecimal amount;

    /**
     * Optional date for the exchange rate.
     * If not provided, the latest available local snapshot is used.
     */

    private LocalDate date;
}
