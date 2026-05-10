package com.banka1.banking_service.account_service.domain;

import jakarta.persistence.Column;
import jakarta.persistence.Entity;
import jakarta.persistence.Table;
import jakarta.validation.constraints.NotBlank;
import lombok.AllArgsConstructor;
import lombok.Getter;
import lombok.NoArgsConstructor;
import lombok.Setter;
import org.hibernate.annotations.CreationTimestamp;

import java.math.BigDecimal;
import java.time.LocalDateTime;

/**
 * JPA entitet koji bilježi svaki obracun naknade za održavanje računa.
 * <p>
 * Služi kao revizorski trag mesečnih odbitaka koje vrši servis za održavanje
 * računa. Svaki zapis je nepromenjiv nakon kreiranja.
 */
@Entity
@Table(
        name = "transaction_record_table"
)
@NoArgsConstructor
@AllArgsConstructor
@Getter
@Setter
public class TransactionRecord extends BaseEntity {

    /** Broj klijentskog računa sa kojeg je naknada skinuta. */
    @NotBlank
    @Column(nullable = false, updatable = false)
    private String accountNumber;

    /** Broj banka-računa u istoj valuti na koji je naknada kreditovana. */
    @NotBlank
    @Column(nullable = false, updatable = false)
    private String bankAccountNumber;

    /** Iznos naknade za održavanje koji je oduzet. */
    @Column(nullable = false, updatable = false)
    private BigDecimal amount;

    /**
     * Kreira novi zapis o transakciji naknade za održavanje.
     *
     * @param accountNumber     broj klijentskog računa sa kojeg je naknada skinuta
     * @param bankAccountNumber broj banka-računa koji je primio naknadu
     * @param amount            iznos naknade za održavanje
     */
    public TransactionRecord(String accountNumber, String bankAccountNumber, BigDecimal amount) {
        this.accountNumber = accountNumber;
        this.bankAccountNumber = bankAccountNumber;
        this.amount = amount;
    }

    /** Datum i vreme kreiranja zapisa. Automatski se postavlja pri čuvanju. */
    @CreationTimestamp
    @Column(name = "created_at", updatable = false)
    private LocalDateTime createdAt;
}
