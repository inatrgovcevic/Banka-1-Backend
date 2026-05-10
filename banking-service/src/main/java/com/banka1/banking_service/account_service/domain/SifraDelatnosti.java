package com.banka1.banking_service.account_service.domain;

import jakarta.persistence.*;
import jakarta.validation.constraints.NotBlank;
import lombok.AccessLevel;
import lombok.Getter;
import lombok.NoArgsConstructor;

import java.util.Collections;
import java.util.HashSet;
import java.util.Set;

/**
 * Imutabilni JPA entitet koji predstavlja sifru delatnosti prema zvanicnoj klasifikaciji.
 * Koristi se za razvrstavanje firmi po delatnosti pri kreiranju poslovnih racuna.
 */
@Entity
@org.hibernate.annotations.Immutable
@Table(
        name = "sifra_delatnosti_table"
)
@Getter
@NoArgsConstructor(access = AccessLevel.PROTECTED)
public class SifraDelatnosti extends BaseEntity {

    /** Jedinstvena sifra delatnosti (npr. "6419"). */
    @NotBlank
    @Column(nullable = false, updatable = false, unique = true)
    private String sifra;

    /** Skup sektora kojima ova delatnost pripada. */
    @ElementCollection
    @CollectionTable(
            name = "sifra_delatnosti_sektori",
            joinColumns = @JoinColumn(name = "sifra_delatnosti_id")
    )
    @Column(name = "sektor", nullable = false)
    private Set<String> sektori = new HashSet<>();

    /** Naziv grane delatnosti (npr. "Finansijska delatnost"). */
    @NotBlank
    @Column(nullable = false, updatable = false)
    private String grana;

    /**
     * Vraca nepromenjiv pogled na skup sektora ove sifre delatnosti.
     *
     * @return nepromenljivi set sektora
     */
    public Set<String> getSektori() {
        return Collections.unmodifiableSet(sektori);
    }
}
