package com.banka1.banking_service.account_service.domain.enums;

import com.banka1.banking_service.account_service.domain.enums.AccountOwnershipType;
import lombok.AllArgsConstructor;
import lombok.Getter;

/**
 * Enumeracija koja definise konkretne podtipove tekuceg (RSD) bankarskog racuna.
 * Svaka vrednost nosi tip vlasnistva i numericki kod koji se ugradjuje u broj racuna.
 */
@AllArgsConstructor
@Getter
public enum AccountConcrete {

    /** Standardni licni tekuci racun. */
    STANDARDNI(AccountOwnershipType.PERSONAL, 11),

    /** Stedni licni racun. */
    STEDNI(AccountOwnershipType.PERSONAL, 13),

    /** Penzionerski licni racun. */
    PENZIONERSKI(AccountOwnershipType.PERSONAL, 14),

    /** Racun za mlade klijente. */
    ZA_MLADE(AccountOwnershipType.PERSONAL, 15),

    /** Studentski racun. */
    ZA_STUDENTE(AccountOwnershipType.PERSONAL, 16),

    /** Racun za nezaposlene klijente. */
    ZA_NEZAPOSLENE(AccountOwnershipType.PERSONAL, 17),

    /** Poslovni racun za drustvo sa ogranicenom odgovornoscu (DOO). */
    DOO(AccountOwnershipType.BUSINESS, 12),

    /** Poslovni racun za akcionarsko drustvo (AD). */
    AD(AccountOwnershipType.BUSINESS, 12),

    /** Racun za fondacije i neprofitne organizacije. */
    FONDACIJA(AccountOwnershipType.BUSINESS, 12);

    /** Tip vlasnistva (PERSONAL ili BUSINESS) koji ovaj podtip zahteva. */
    private final AccountOwnershipType accountOwnershipType;

    /** Numericki kod koji se ugradjuje u generisani broj racuna. */
    private final int val;
}
