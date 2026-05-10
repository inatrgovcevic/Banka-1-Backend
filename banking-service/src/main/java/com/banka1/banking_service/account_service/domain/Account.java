package com.banka1.banking_service.account_service.domain;


import com.banka1.banking_service.account_service.domain.enums.AccountOwnershipType;
import com.banka1.banking_service.account_service.domain.enums.Status;
import jakarta.persistence.*;
import jakarta.validation.constraints.DecimalMin;
import jakarta.validation.constraints.Email;
import jakarta.validation.constraints.NotBlank;
import lombok.AllArgsConstructor;
import lombok.Getter;
import lombok.NoArgsConstructor;
import lombok.Setter;
import org.hibernate.annotations.CreationTimestamp;

import java.math.BigDecimal;
import java.time.LocalDate;
import java.time.LocalDateTime;

/**
 * Apstraktni JPA entitet koji predstavlja bankarski racun.
 * Koristi strategiju nasledjivanja SINGLE_TABLE — sve vrste racuna
 * ({@link CheckingAccount}, {@link FxAccount}) dele istu tabelu {@code account_table}
 * i razlikuju se po koloni {@code account_type}.
 */
@Entity
@Table(
        name = "account_table",
        indexes = {
            @Index(name = "idx_account_vlasnik", columnList = "vlasnik"),
            @Index(name = "idx_account_broj", columnList = "broj_racuna"),
            @Index(name = "idx_account_company", columnList = "company_id"),
            @Index(name = "idx_account_ime_vlasnika", columnList = "ime_vlasnika_racuna"),
            @Index(name = "idx_account_prezime_vlasnika", columnList = "prezime_vlasnika_racuna"),
            @Index(name = "idx_account_ime_prezime_vlasnika", columnList = "ime_vlasnika_racuna, prezime_vlasnika_racuna")
        }
)
@Inheritance(strategy = InheritanceType.SINGLE_TABLE)
@DiscriminatorColumn(name = "account_type", discriminatorType = DiscriminatorType.STRING)
@NoArgsConstructor
@AllArgsConstructor
@Getter
@Setter
//todo da li staviti datum isteka na null
public abstract class Account extends BaseEntity{
    /** Jedinstveni 19-cifreni broj racuna koji se generise pri kreiranju. */
    @NotBlank
    @Column(nullable = false,unique = true,updatable = false)
    private String brojRacuna;

    /** Ime vlasnika racuna (preuzeto iz korisnickog servisa pri kreiranju). */
    @NotBlank
    @Column(nullable = false)
    private String imeVlasnikaRacuna;

    /** Prezime vlasnika racuna (preuzeto iz korisnickog servisa pri kreiranju). */
    @NotBlank
    @Column(nullable = false)
    private String prezimeVlasnikaRacuna;

    /** Email adresa vlasnika racuna, koristi se za slanje notifikacija. */
    @Email
    @Column
    private String email;

    /** Korisnicko ime vlasnika racuna, koristi se za slanje notifikacija. */
    @Column
    private String username;

    /** Korisnicki naziv racuna koji vlasnik moze menjati. */
    @NotBlank
    @Column(nullable = false)
    private String nazivRacuna;

    /** ID klijenta-vlasnika racuna (referencira korisnika iz korisnickog servisa). */
    @Column(nullable = false)
    private Long vlasnik;

    /** Ukupno stanje racuna (ukljucujuci rezervisana sredstva). */
    @Column(nullable = false)
    private BigDecimal stanje= BigDecimal.ZERO;

    /** Raspolozivo stanje racuna (bez rezervisanih sredstava). */
    @DecimalMin(value = "0.00", inclusive = true)
    @Column(nullable = false)
    private BigDecimal raspolozivoStanje= BigDecimal.ZERO;

    /** ID zaposlenog koji je kreirao racun. */
    @Column(nullable = false)
    private Long zaposlen;

    /** Datum i vreme kreiranja racuna, automatski se postavlja. */
    @CreationTimestamp
    @Column(name = "datum_i_vreme_kreiranja",nullable = false,updatable = false)
    private LocalDateTime datumIVremeKreiranja;

    /** Datum isteka racuna. Ako je prosao, racun se ne moze koristiti za transakcije. */
    private LocalDate datumIsteka;

    /** Valuta racuna. Za tekuce racune mora biti RSD, za FX racune ne sme biti RSD. */
    @ManyToOne(optional = false, fetch = FetchType.LAZY)
    @JoinColumn(name = "currency_id", nullable = false)
    private Currency currency;

    /** Status racuna (ACTIVE ili INACTIVE). */
    @Column(nullable = false)
    @Enumerated(EnumType.STRING)
    private Status status=Status.ACTIVE;

    /** Dnevni limit trosenja na racunu. */
    @DecimalMin(value = "0.00", inclusive = true)
    private BigDecimal dnevniLimit;

    /** Mesecni limit trosenja na racunu. */
    @DecimalMin(value = "0.00", inclusive = true)
    private BigDecimal mesecniLimit;

    /** Ukupno potroseno u tekucem danu, resetuje se svakodnevno. */
    @DecimalMin(value = "0.00", inclusive = true)
    @Column(nullable = false)
    private BigDecimal dnevnaPotrosnja=BigDecimal.ZERO;

    /** Ukupno potroseno u tekucem mesecu, resetuje se mesecno. */
    @DecimalMin(value = "0.00", inclusive = true)
    @Column(nullable = false)
    private BigDecimal mesecnaPotrosnja=BigDecimal.ZERO;

    /** Firma vezana za racun, popunjava se samo za poslovne racune. */
    @ManyToOne(fetch = FetchType.LAZY)
    @JoinColumn(name = "company_id")
    private Company company;


    /**
     * Validira uskladjenost tipa vlasnistva i prisustva firme.
     * <p>
     * Pravila:
     * <ul>
     *   <li>Tip vlasnistva mora biti postavljen (nije null)</li>
     *   <li>BUSINESS računi moraju imati firmu</li>
     *   <li>PERSONAL računi ne smeju imati firmu</li>
     * </ul>
     * Poziva se iz {@code @PrePersist}/{@code @PreUpdate} metoda podklasa.
     *
     * @param ownershipType tip vlasnistva računa
     * @throws IllegalStateException ako tip vlasnistva nije zadovoljen ili firma nedostaje/ne treba
     */
    protected void validacija(AccountOwnershipType ownershipType) {
        if (ownershipType == null) {
            throw new IllegalStateException("Ownership type is required");
        }

        if (ownershipType == AccountOwnershipType.BUSINESS && this.getCompany() == null) {
            throw new IllegalStateException("Company is required for BUSINESS account");
        }

        if (ownershipType == AccountOwnershipType.PERSONAL && this.getCompany() != null) {
            throw new IllegalStateException("Company must be null for PERSONAL account");
        }
    }


}
