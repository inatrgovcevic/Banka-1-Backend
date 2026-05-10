package com.banka1.banking_service.account_service.dto.request;

import jakarta.validation.constraints.NotBlank;
import jakarta.validation.constraints.NotNull;
import jakarta.validation.constraints.Pattern;
import lombok.AllArgsConstructor;
import lombok.Getter;
import lombok.NoArgsConstructor;
import lombok.Setter;

/**
 * DTO za podatke o firmi (pravnom licu) pri kreiranju poslovnog racuna.
 * <p>
 * Sadrzi osnovne identifikacione i poreske podatke potrebne za registraciju
 * poslovnog racuna.
 * <p>
 * Validacija:
 * <ul>
 *   <li>Naziv mora biti popunjen</li>
 *   <li>Matični broj mora biti tačno 8 cifara</li>
 *   <li>Poreski broj mora biti tačno 9 cifara</li>
 *   <li>Šifra delatnosti mora biti popunjena</li>
 *   <li>ID vlasnika mora biti postavljen</li>
 * </ul>
 */
@NoArgsConstructor
@AllArgsConstructor
@Getter
@Setter
public class FirmaDto {
    /**
     * Naziv firme (pravnog lica).
     */
    @NotBlank(message = "Unesi naziv")
    private String naziv;

    /**
     * Matični broj firme - jedinstveni 8-cifreni identifikator.
     */
    @NotBlank(message = "Unesi maticni broj")
    @Pattern(regexp = "^\\d{8}$", message = "Maticni broj mora imati tacno 8 cifara")
    private String maticniBroj;

    /**
     * Poreski broj firme - jedinstveni 9-cifreni identifikator za poreske svrhe.
     */
    @NotBlank(message = "Unesi poreski broj")
    @Pattern(regexp = "^\\d{9}$", message = "Poreski broj mora imati tacno 9 cifara")
    private String poreskiBroj;

    /**
     * Šifra delatnosti firme - klasifikacija tipa poslovanja.
     */
    @NotBlank(message = "Unesi sifru delatnosti")
    private String sifraDelatnosti;

    /**
     * Adresa firme (opciono).
     */
    private String adresa;

    /**
     * ID vlasnika firme (klijenta koji je osnovao firmu).
     */
    @NotNull(message = "Unesi vlasnika")
    private Long vlasnik;
}
