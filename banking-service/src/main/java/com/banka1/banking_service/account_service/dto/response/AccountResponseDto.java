package com.banka1.banking_service.account_service.dto.response;

import com.banka1.banking_service.account_service.domain.Account;
import com.banka1.banking_service.account_service.domain.CheckingAccount;
import com.banka1.banking_service.account_service.domain.FxAccount;
import lombok.AllArgsConstructor;
import lombok.Getter;
import lombok.NoArgsConstructor;
import lombok.Setter;

import java.math.BigDecimal;

/**
 * DTO za skraćeni odgovor sa osnovnim podacima o bankarskom računu.
 * <p>
 * Koristi se u slučajevima gde se ne trebaju svi detalji kao što su limitri i trošenja.
 * Sadrži samo osnovne identifikacione i finansijske podatke.
 * <p>
 * Tipična upotreba: lista svih računa korisnika, pregled računa, itd.
 */
@NoArgsConstructor
@AllArgsConstructor
@Getter
@Setter
public class AccountResponseDto {
    /**
     * Primarni kljuc racuna u bazi. Frontend i drugi servisi koriste ovu vrednost kao
     * jedinstveni numericki identifikator (npr. order-service prosledjuje ga kao
     * {@code accountId} u BUY/SELL request body-ju). Bez ovog polja, frontend je
     * morao da hash-uje {@code brojRacuna} u 32-bit integer da bi imao "id", sto je
     * proizvodilo vrednosti koje ne mapiraju ni na PK ni na broj racuna i rusilo
     * naredne {@code /internal/accounts/id/...} pozive (vidi GHI #199).
     */
    private Long id;

    /** Naziv koji je korisnik dao za svoj račun. */
    private String nazivRacuna;

    /** Jedinstveni 18-cifreni broj računa. */
    private String brojRacuna;

    /** Iznos dostupan za trošenje na računu. */
    private BigDecimal raspolozivoStanje;

    /** Kod valute računa (USD, EUR, RSD, itd.). */
    private String currency;

    /** Kategorija računa (CHECKING ili FOREIGN_CURRENCY). */
    private String accountCategory;

    /** Tip vlasnistva računa (PERSONAL ili BUSINESS). */
    private String accountType;

    /** Podtip računa (za tekuće račune: STANDARDNI, ZA_MLADE, itd.). */
    private String subtype;

    /**
     * Kreira DTO od {@link Account} entiteta.
     * <p>
     * Automatski mapira osnovne polje i odrđuje tip računa na osnovu tipa entiteta.
     *
     * @param account entitet iz baze
     */
    public AccountResponseDto(Account account) {
        this.id = account.getId();
        this.nazivRacuna = account.getNazivRacuna();
        this.brojRacuna = account.getBrojRacuna();
        this.raspolozivoStanje = account.getRaspolozivoStanje();
        this.currency = account.getCurrency() != null ? account.getCurrency().getOznaka().name() : null;
        if (account instanceof CheckingAccount ca) {
            this.accountCategory = "CHECKING";
            this.accountType = ca.getAccountConcrete().getAccountOwnershipType().name();
            this.subtype = ca.getAccountConcrete().name();
        } else if (account instanceof FxAccount fa) {
            this.accountCategory = "FOREIGN_CURRENCY";
            this.accountType = fa.getAccountOwnershipType().name();
            this.subtype = null;
        }
    }
}
