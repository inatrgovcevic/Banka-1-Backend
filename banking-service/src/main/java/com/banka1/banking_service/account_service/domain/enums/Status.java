package com.banka1.banking_service.account_service.domain.enums;

/**
 * Enumeracija koja predstavlja status bankarskog računa ili valute.
 * <p>
 * Statusom se kontroliše da li je entitet dostupan za korišćenje u sistemu.
 * Može se primenjivati na račune, valute i druge entitete.
 */
public enum Status {
    /** Entitet je aktivan i dostupan za korišćenje u sistemu. */
    ACTIVE,

    /** Entitet je deaktiviran i nije dostupan za nove operacije. */
    INACTIVE
}
