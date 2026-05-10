package com.banka1.banking_service.transaction_service.dto.request;

import lombok.Getter;
import lombok.Setter;

import java.math.BigDecimal;
import java.time.LocalDate;

/**
 * DTO for querying currency conversion details.
 * Contains information about the source and target currencies and the amount to be converted.
 */
@Getter
@Setter
public class ConversionQueryDto {

    private static final String SUPPORTED_CURRENCY_REGEX = "^(?i)(RSD|EUR|CHF|USD|GBP|JPY|CAD|AUD)$";
    private static final String SUPPORTED_CURRENCY_MESSAGE =
            "Supported currencies are RSD, EUR, CHF, USD, GBP, JPY, CAD and AUD.";

    /** Source currency code. */
    private String fromCurrency;

    /** Target currency code. */
    private String toCurrency;

    /** Amount to be converted. */
    private BigDecimal amount;

    /** Conversion rate to be applied. */
    private LocalDate date;
}
