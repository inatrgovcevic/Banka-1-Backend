package com.banka1.banking_service.transfer_service.domain;

import jakarta.persistence.*;
import lombok.*;

import java.math.BigDecimal;
import java.time.Instant;

/**
 * Entitet koji predstavlja trajni zapis o izvršenom prenosu sredstava (transferu).
 * Sadrži sve relevantne podatke o transakciji, uključujući račune, iznose, kurseve i metapodatke.
 */
@Entity
@Table(name = "transfers")
@Getter
@Setter
@NoArgsConstructor
@AllArgsConstructor
@Builder
// @SQLDelete(sql = "UPDATE transfers SET deleted = true WHERE id = ?") // Opciono, ako se dogovorimo da kada bude delete, on bude soft, otkomenarisati ovo
public class Transfer {

    /** Jedinstveni identifikator zapisa u bazi. */
    @Id
    @GeneratedValue(strategy = GenerationType.IDENTITY)
    private Long id;

    /** Jedinstveni poslovni broj naloga generisan prilikom kreiranja transfera. */
    @Column(nullable = false, unique = true)
    private String orderNumber;

    /** ID klijenta (vlasnika oba računa) koji je inicirao prenos. */
    @Column(nullable = false)
    private Long clientId;

    /** Broj računa sa kojeg se sredstva skidaju. */
    @Column(nullable = false)
    private String fromAccountNumber;

    /** Broj računa na koji se sredstva uplaćuju. */
    @Column(nullable = false)
    private String toAccountNumber;

    /** Originalni iznos transfera u valuti izvornog računa. */
    @Column(nullable = false)
    private BigDecimal initialAmount;

    /** Finalni iznos koji se uplaćuje na ciljni račun nakon konverzije i provizija. */
    @Column(nullable = false)
    private BigDecimal finalAmount;

    /** Primenjeni kurs menjačnice (null ako su valute računa iste). */
    private BigDecimal exchangeRate;

    /** Naplaćena provizija za transfer ili konverziju. */
    @Column(nullable = false)
    private BigDecimal commission;

    /** Vreme kada je transfer potvrđen i izvršen. */
    @Column(nullable = false)
    private Instant timestamp;

    /** ID sesije verifikacionog servisa korišćen za idempotenciju. */
    @Column(nullable = false, unique = true)
    private String verificationSessionId;

    /** Optimizmička kontrola verzije za konkurentne izmene. */
    @Version
    private Long version;

    /** Logičko brisanje (soft delete) - podrazumevano false. */
    @Builder.Default
    private Boolean deleted = false;

    /** Vreme kreiranja entiteta u bazi. */
    private Instant createdAt;

    /** Vreme poslednjeg ažuriranja entiteta u bazi. */
    private Instant updatedAt;

    /** Metoda koja postavlja početne vremenske oznake pre čuvanja u bazu. */
    @PrePersist
    protected void onCreate() {
        createdAt = Instant.now();
        updatedAt = Instant.now();
        if (timestamp == null) timestamp = Instant.now();
    }

    /** Metoda koja ažurira vremensku oznaku pre svakog ažuriranja u bazi. */
    @PreUpdate
    protected void onUpdate() {
        updatedAt = Instant.now();
    }
}