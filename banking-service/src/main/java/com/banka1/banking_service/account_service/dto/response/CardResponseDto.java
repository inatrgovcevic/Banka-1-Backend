package com.banka1.banking_service.account_service.dto.response;

import lombok.AllArgsConstructor;
import lombok.Getter;
import lombok.NoArgsConstructor;
import lombok.Setter;

/**
 * DTO za odgovor sa informacijama o bankovskoj kartici.
 * <p>
 * Sadrži osnovne informacije o kartici vezanoj za račun, uključujući identifikaciju,
 * tip, status i datum isteka. Koristi se za prikaz kartice u detalјima računa ili
 * pri pretrazi/upravljanju karticama.
 * <p>
 * Napomena: Broj kartice je delimično maskiran iz bezbednosnih razloga (prikazana su samo
 * poslednja 4 karaktera).
 */
@NoArgsConstructor
@AllArgsConstructor
@Getter
@Setter
public class CardResponseDto {
    /**
     * Jedinstveni identifikator kartice u bazi podataka.
     */
    private Long id;

    /**
     * Broj kartice, delimično maskiran iz bezbednosnih razloga.
     * Format: XXXX-XXXX-XXXX-1234 (prikazana samo poslednja 4 cifre).
     */
    private String cardNumber;

    /**
     * Tip kartice (npr. "DEBIT", "CREDIT", "PREPAID").
     */
    private String cardType;

    /**
     * Status kartice (npr. "ACTIVE", "BLOCKED", "EXPIRED", "CANCELLED").
     */
    private String status;

    /**
     * Datum isteka kartice u formatu MM/YY.
     */
    private String expiryDate;

    /**
     * Broj računa na koji je kartica vezana (19-cifreni broj).
     */
    private String accountNumber;
}
