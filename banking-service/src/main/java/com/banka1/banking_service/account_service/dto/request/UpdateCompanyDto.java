package com.banka1.banking_service.account_service.dto.request;

import jakarta.validation.constraints.NotBlank;
import lombok.AllArgsConstructor;
import lombok.Getter;
import lombok.NoArgsConstructor;
import lombok.Setter;

/**
 * DTO za zahtev azuriranja podataka o firmi.
 * <p>
 * Omogucava zaposlenim da izmene osnovne informacije o registrovanoj firmi
 * (naziv, šifru delatnosti, adresu i vlasnika).
 * <p>
 * Validacija:
 * <ul>
 *   <li>Naziv mora biti popunjen</li>
 *   <li>Šifra delatnosti mora biti popunjena i postojeća u sistemu</li>
 *   <li>Adresa je opciona</li>
 *   <li>Vlasnik je opciono</li>
 * </ul>
 */
@Getter
@Setter
@NoArgsConstructor
@AllArgsConstructor
public class UpdateCompanyDto {
    /**
     * Novi naziv firme.
     */
    @NotBlank(message = "Naziv ne sme biti prazan")
    private String naziv;

    /**
     * Nova šifra delatnosti firme.
     * <p>
     * Mora postojati u sistemu, inače će se baciti greška.
     */
    @NotBlank(message = "Sifra delatnosti ne sme biti prazna")
    private String sifraDelatnosti;

    /**
     * Nova adresa firme (opciono).
     */
    private String adresa;

    /**
     * Novi vlasnik firme (opciono).
     */
    private Long vlasnik;
}
