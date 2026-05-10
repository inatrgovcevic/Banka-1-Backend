package com.banka1.banking_service.account_service.domain;

import jakarta.persistence.*;
import jakarta.validation.constraints.NotBlank;
import lombok.AllArgsConstructor;
import lombok.Getter;
import lombok.NoArgsConstructor;
import lombok.Setter;

/**
 * JPA entitet koji predstavlja pravno lice (firmu) vezano za poslovni bankarski racun.
 * Sadrzi osnovne podatke o firmi: naziv, maticni i poreski broj, sifru delatnosti i adresu.
 */
@Entity
@Table(
        name = "company_table"
)
@NoArgsConstructor
@AllArgsConstructor
@Getter
@Setter
public class Company extends BaseEntity {

    /** Naziv firme. */
    @NotBlank
    @Column(nullable = false)
    private String naziv;

    /** Maticni broj firme — jedinstveni 8-cifreni identifikator pravnog lica. */
    @NotBlank
    @Column(nullable = false, unique = true)
    private String maticni_broj;

    /** Poreski identifikacioni broj (PIB) — jedinstveni 9-cifreni broj. */
    @NotBlank
    @Column(nullable = false, unique = true)
    private String poreski_broj;

    /** Sifra delatnosti firme prema klasifikaciji delatnosti. */
    @ManyToOne(optional = false, fetch = FetchType.LAZY)
    @JoinColumn(name = "sifra_delatnosti_id", nullable = false)
    private SifraDelatnosti sifraDelatnosti;

    /** Adresa sedista firme (opciono polje). */
    private String adresa;

    /** ID klijenta koji je vlasnik firme (referencira korisnika iz korisnickog servisa). */
    @Column(nullable = false)
    private Long vlasnik;
}
