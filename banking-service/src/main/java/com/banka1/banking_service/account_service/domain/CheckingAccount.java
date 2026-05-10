package com.banka1.banking_service.account_service.domain;

import com.banka1.banking_service.account_service.domain.Account;
import com.banka1.banking_service.account_service.domain.enums.AccountConcrete;
import com.banka1.banking_service.account_service.domain.enums.CurrencyCode;
import jakarta.persistence.*;
import lombok.Getter;
import lombok.NoArgsConstructor;
import lombok.Setter;

import java.math.BigDecimal;

/**
 * JPA entitet koji predstavlja tekući bankarski račun denominovan iskljucivo u RSD.
 * <p>
 * Nasleđuje {@link Account} i dodaje specifična polja: vrstu tekućeg računa
 * i mesečnu naknadu za održavanje. Svi tekući računi su u RSD valuti.
 */
@NoArgsConstructor
@Getter
@Setter
@Entity
@DiscriminatorValue("CHECKING")
public class CheckingAccount extends Account {

    /** Konkretan podtip tekućeg računa (lični, štedni, poslovni, itd.). */
    @Column(nullable = false)
    @Enumerated(EnumType.STRING)
    private AccountConcrete accountConcrete;

    /** Mesečna naknada za održavanje računa u RSD. Vrednost 0 znači bez naknade. */
    private BigDecimal odrzavanjeRacuna = BigDecimal.ZERO;

    /**
     * Kreira tekuci racun zadatog podtipa.
     *
     * @param accountConcrete vrsta tekuceg racuna
     */
    public CheckingAccount(AccountConcrete accountConcrete) {
        this.accountConcrete = accountConcrete;
    }

    /**
     * JPA hook koji se poziva pre upisivanja i ažuriranja entiteta.
     * <p>
     * Proverava da li je tip računa postavljen, da li je valuta RSD
     * i da li je tip vlasnistva konzistentan sa prisusstvom firme.
     *
     * @throws IllegalStateException ako valuta nije RSD, tip računa nije postavljen
     *                                  ili podaci nisu konzistentni
     */
    @PrePersist
    @PreUpdate
    private void validate() {
        if (accountConcrete == null) {
            throw new IllegalStateException("accountConcrete ne sme biti null");
        }
        validacija(accountConcrete.getAccountOwnershipType());
        if (getCurrency() == null || getCurrency().getOznaka() != CurrencyCode.RSD) {
            throw new IllegalStateException("Tekuci racun mora biti u RSD");
        }
    }

    /**
     * Postavlja valutu računa sa validacijom.
     * <p>
     * Baca izuzetak ako valuta nije RSD jer tekući računi mogu biti
     * samo u domaćoj valuti.
     *
     * @param currency valuta koja se postavlja
     * @throws IllegalArgumentException ako valuta nije RSD
     */
    @Override
    public void setCurrency(Currency currency) {
        if (currency.getOznaka() != CurrencyCode.RSD)
            throw new IllegalArgumentException("Tekuci racun mora biti u RSD");
        super.setCurrency(currency);
    }
}
