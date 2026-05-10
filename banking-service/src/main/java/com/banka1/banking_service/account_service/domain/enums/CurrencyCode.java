package com.banka1.banking_service.account_service.domain.enums;

/**
 * Enumeracija ISO 4217 kodova valuta podržanih od strane banke.
 * <p>
 * Koristi se kao diskriminator kod pri čuvanju valuta u bazi podataka.
 * Redosled konstanti ne treba menjati jer se koristi u Liquibase migracijama.
 */
public enum CurrencyCode {
    /** Srpski dinar */
    RSD,
    /** Evro */
    EUR,
    /** Švajcarski franak */
    CHF,
    /** Američki dolar */
    USD,
    /** Britanska funta */
    GBP,
    /** Japanski jen */
    JPY,
    /** Kanadski dolar */
    CAD,
    /** Australijski dolar */
    AUD
}
