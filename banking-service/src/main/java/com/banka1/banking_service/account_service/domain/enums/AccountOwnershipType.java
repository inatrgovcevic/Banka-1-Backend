package com.banka1.banking_service.account_service.domain.enums;

import lombok.AllArgsConstructor;
import lombok.Getter;

/**
 * Enumeracija koja definise tip vlasnistva bankarskog racuna.
 * Svaka vrednost nosi numericki kod koji se ugradjuje u broj racuna.
 */
@AllArgsConstructor
@Getter
public enum AccountOwnershipType {

    /** Licni racun fizickog lica. */
    PERSONAL(21),

    /** Poslovni racun pravnog lica (firme). */
    BUSINESS(22);

    /** Numericki kod koji se ugradjuje u generisani broj racuna. */
    private final int val;
}
