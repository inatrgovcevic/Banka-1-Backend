package com.banka1.banking_service.credit_service.dto.response;

import com.banka1.banking_service.credit_service.domain.enums.CurrencyCode;
import lombok.AllArgsConstructor;
import lombok.Getter;
import lombok.NoArgsConstructor;
import lombok.Setter;

/**
 * DTO containing information about accounts for a transaction.
 * Used to transfer account details between services.
 */
@AllArgsConstructor
@Getter
@Setter
@NoArgsConstructor
public class InfoResponseDto {
    /** The currency code of the source account. */
    private CurrencyCode fromCurrencyCode;

    /** The currency code of the destination account. */
    private CurrencyCode toCurrencyCode;

    /** The owner ID of the source account. */
    private Long fromVlasnik;

    /** The owner ID of the destination account. */
    private Long toVlasnik;

    /** The email address of the source account owner. */
    private String fromEmail;

    /** The username of the source account owner. */
    private String fromUsername;
}
