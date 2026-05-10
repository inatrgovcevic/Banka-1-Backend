package com.banka1.banking_service.account_service.domain;

import com.banka1.banking_service.account_service.domain.BaseEntity;
import com.banka1.banking_service.account_service.domain.enums.CurrencyCode;
import com.banka1.banking_service.account_service.domain.enums.Status;
import jakarta.persistence.*;
import jakarta.validation.constraints.NotBlank;
import lombok.AccessLevel;
import lombok.Getter;
import lombok.NoArgsConstructor;

import java.util.Collections;
import java.util.HashSet;
import java.util.Set;

/**
 * Nepromenljivi JPA entitet koji predstavlja valutu podržanu od strane banke.
 * <p>
 * Podaci o valutama se učitavaju putem Liquibase seed migracija pri pokretanju
 * i ne menjaju se u runtime-u. Entitet je označen kao {@code @Immutable} da bi
 * Hibernate znao da nema potrebe za praćenjem izmena.
 */
@Entity
@org.hibernate.annotations.Immutable
@Table(
        name = "currency_table"
)
@Getter
@NoArgsConstructor(access = AccessLevel.PROTECTED)
public class Currency extends BaseEntity {

    /** Pun naziv valute (npr. "Srpski dinar"). */
    @NotBlank
    @Column(nullable = false, updatable = false)
    private String naziv;

    /** ISO 4217 kod valute (npr. RSD, EUR, USD). */
    @Enumerated(EnumType.STRING)
    @Column(nullable = false, updatable = false, unique = true)
    private CurrencyCode oznaka;

    /** Simbol valute za prikaz (npr. "din", "€", "$"). */
    @NotBlank
    @Column(nullable = false, updatable = false, unique = true)
    private String simbol;

    /** Skup zemalja u kojima se ova valuta koristi (npr. "Srbija", "Švajcarska"). */
    @ElementCollection
    @CollectionTable(name = "currency_countries", joinColumns = @JoinColumn(name = "currency_id"))
    @Column(name = "country", nullable = false)
    private Set<String> countries = new HashSet<>();

    /** Kratak opisni tekst o valuti (npr. "Valuta Srbije"). */
    @NotBlank
    @Column(nullable = false, updatable = false)
    private String opis;

    /** Status valute — samo ACTIVE valute mogu se koristiti za kreiranje novih računa. */
    @Enumerated(EnumType.STRING)
    @Column(nullable = false, updatable = false)
    private Status status;

    /**
     * Kreira novu valutu sa svim obaveznim podacima.
     * <p>
     * Koristi se tokom Liquibase seed migracija za inicijalizaciju valuta.
     *
     * @param naziv     pun naziv valute (npr. "Srpski dinar")
     * @param oznaka    ISO 4217 kod valute (npr. RSD, EUR, USD)
     * @param simbol    simbol valute za prikaz (npr. "din", "€", "$")
     * @param countries skup zemalja u kojima se valuta koristi
     * @param opis      kratak opisni tekst o valuti
     * @param status    inicijalni status valute (ACTIVE ili INACTIVE)
     */
    public Currency(String naziv, CurrencyCode oznaka, String simbol, Set<String> countries, String opis, Status status) {
        this.naziv = naziv;
        this.oznaka = oznaka;
        this.simbol = simbol;
        this.countries = countries;
        this.opis = opis;
        this.status = status;
    }

    /**
     * Vraća nepromenjiv pogled na skup zemalja ove valute.
     * <p>
     * Vraćeni set je zaštićen od promena, što održava integritet entiteta.
     *
     * @return nepromenljivi skup zemalja
     */
    public Set<String> getCountries() {
        return Collections.unmodifiableSet(countries);
    }
}
