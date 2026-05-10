package com.banka1.banking_service.account_service.dto.response;

import com.banka1.banking_service.account_service.domain.Company;
import lombok.Getter;
import lombok.NoArgsConstructor;
import lombok.Setter;

/**
 * DTO za odgovor sa podacima o firmi (pravnom licu).
 * <p>
 * Koristi se kada se vraćaju informacije o registrovanoj firmi
 * koja je vezana za poslovni bankarski račun.
 */
@Getter
@Setter
@NoArgsConstructor
public class CompanyResponseDto {
    /** Jedinstveni identifikator firme u bazi. */
    private Long id;

    /** Naziv firme. */
    private String naziv;

    /** Matični broj firme (8 cifara). */
    private String maticniBroj;

    /** Poreski broj firme (9 cifara). */
    private String poreskiBroj;

    /** Šifra delatnosti (klasifikacija poslovanja). */
    private String sifraDelatnosti;

    /** Adresa sedišta firme. */
    private String adresa;

    /** ID vlasnika firme (klijenta). */
    private Long vlasnik;

    /**
     * Kreira DTO od {@link Company} entiteta.
     *
     * @param company entitet firme iz baze
     */
    public CompanyResponseDto(Company company) {
        this.id = company.getId();
        this.naziv = company.getNaziv();
        this.maticniBroj = company.getMaticni_broj();
        this.poreskiBroj = company.getPoreski_broj();
        this.sifraDelatnosti = company.getSifraDelatnosti() != null ? company.getSifraDelatnosti().getSifra() : null;
        this.adresa = company.getAdresa();
        this.vlasnik = company.getVlasnik();
    }
}
