package com.banka1.banking_service.account_service.domain;

import com.banka1.banking_service.account_service.domain.Account;
import com.banka1.banking_service.account_service.domain.enums.AccountOwnershipType;
import com.banka1.banking_service.account_service.domain.enums.CurrencyCode;
import jakarta.persistence.*;
import lombok.AllArgsConstructor;
import lombok.Getter;
import lombok.NoArgsConstructor;
import lombok.Setter;

/**
 * JPA entitet koji predstavlja devizni (FX) bankarski račun.
 * <p>
 * Nasleđuje {@link Account} i nije dozvoljen u RSD valuti.
 * Svi devizni računi moraju biti u stranoj valuti.
 * Ima tip vlasnistva (lični ili poslovni).
 */
@NoArgsConstructor
@Getter
@Setter
@Entity
@DiscriminatorValue("FX")
@AllArgsConstructor
public class FxAccount extends Account {

    /** Tip vlasnistva deviznog računa (PERSONAL ili BUSINESS). */
    @Enumerated(EnumType.STRING)
    @Column(nullable = false)
    private AccountOwnershipType accountOwnershipType;

    /**
     * JPA hook koji se poziva pre upisivanja i ažuriranja entiteta.
     * <p>
     * Proverava da je tip vlasnistva postavljen, da valuta nije RSD
     * i da je tip vlasnistva konzistentan sa prisusstvom firme.
     *
     * @throws IllegalStateException ako valuta jeste RSD, tip vlasnistva nije postavljen
     *                                  ili podaci nisu konzistentni
     */
    @PrePersist
    @PreUpdate
    private void validate() {
        if (accountOwnershipType == null) {
            throw new IllegalStateException("accountOwnershipType ne sme biti null");
        }
        validacija(accountOwnershipType);
        if (getCurrency() == null || getCurrency().getOznaka() == CurrencyCode.RSD) {
            throw new IllegalStateException("Devizni racun ne sme biti u RSD");
        }
    }

    /**
     * Postavlja valutu računa sa validacijom.
     * <p>
     * Baca izuzetak ako je valuta RSD jer devizni računi mogu biti
     * samo u stranoj valuti.
     *
     * @param currency valuta koja se postavlja
     * @throws IllegalArgumentException ako je valuta RSD
     */
    public void setCurrency(Currency currency) {
        if (currency.getOznaka() == CurrencyCode.RSD)
            throw new IllegalArgumentException("Devizni racun ne sme biti u RSD");
        super.setCurrency(currency);
    }
}
