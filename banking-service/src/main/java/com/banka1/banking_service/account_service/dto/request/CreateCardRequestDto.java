package com.banka1.banking_service.account_service.dto.request;

import lombok.AllArgsConstructor;
import lombok.Getter;
import lombok.NoArgsConstructor;
import lombok.Setter;

import java.time.LocalDate;

/**
 * DTO za zahtev kreiranja nove bankovske kartice.
 * <p>
 * Sadrži sve potrebne informacije za kreiranje kartice vezane za postojeći račun.
 * Podatke o računu (broj, tip, valuta) korisnik prosleđuje zajedno sa informacijama
 * o vlasniku kartice. Ova DTO se koristi kao intermedijar između servisima.
 * <p>
 * Napomena: Sve vrednosti očekuju da budu validne vrednosti iz bankovnog sistema.
 */
@NoArgsConstructor
@AllArgsConstructor
@Getter
@Setter
public class CreateCardRequestDto {
    /**
     * ID klijenta kojem pripada kartica.
     */
    private Long clientId;

    /**
     * Broj računa na koji se kartica vezuje (19-cifreni broj).
     */
    private String accountNumber;

    /**
     * Naziv računa (npr. "Moj tekući račun").
     */
    private String accountName;

    /**
     * Kod valute računa (npr. "RSD", "EUR", "USD").
     */
    private String accountCurrency;

    /**
     * Kategorija računa (npr. "CHECKING", "FOREIGN_CURRENCY").
     */
    private String accountCategory;

    /**
     * Tip vlasnistva računa (npr. "PERSONAL", "BUSINESS").
     */
    private String accountType;

    /**
     * Podtip računa, primenjuje se za tekuće račune (npr. "STANDARDNI", "ZA_MLADE").
     */
    private String accountSubtype;

    /**
     * Ime vlasnika kartice.
     */
    private String ownerFirstName;

    /**
     * Prezime vlasnika kartice.
     */
    private String ownerLastName;

    /**
     * Email adresa vlasnika kartice (za komunikaciju o kartici).
     */
    private String ownerEmail;

    /**
     * Korisničko ime vlasnika kartice u bankovnom sistemu.
     */
    private String ownerUsername;

    /**
     * Datum isteka računa (ako je primenjiv) na kojeg se vezuje kartica.
     */
    private LocalDate accountExpirationDate;
}

