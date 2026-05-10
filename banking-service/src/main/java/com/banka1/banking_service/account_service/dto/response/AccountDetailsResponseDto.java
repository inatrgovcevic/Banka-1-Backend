package com.banka1.banking_service.account_service.dto.response;

import com.banka1.banking_service.account_service.domain.Account;
import com.banka1.banking_service.account_service.domain.CheckingAccount;
import com.banka1.banking_service.account_service.domain.Company;
import com.banka1.banking_service.account_service.domain.FxAccount;
import lombok.Getter;
import lombok.Setter;

import java.math.BigDecimal;
import java.time.LocalDate;
import java.time.LocalDateTime;
import java.util.List;

/**
 * DTO za odgovor sa detaljnim informacijama o bankarskom računu.
 * <p>
 * Sadrži sve relevantne informacije o računu uključujući:
 * <ul>
 *   <li>Identifikacione podatke (broj, naziv, vlasnik)</li>
 *   <li>Finansijske podatke (stanje, raspoloživo stanje, limitri, trošenja)</li>
 *   <li>Statusne podatke (status, valuta, datum kreiranja)</li>
 *   <li>Podatke o firmi (ako je to poslovni račun)</li>
 *   <li>Kartice vezane za račun</li>
 * </ul>
 * <p>
 * Koristi se u svim odgovorima gde klijent traži detaljne informacije o računu.
 * Automatski mapira entitete {@link Account}, {@link CheckingAccount} i {@link FxAccount}
 * kroz custom konstruktor sa detektovanjem tipa računa.
 * <p>
 * Primer: GET /accounts/{id} vraća DTO sa svim detaljima računa i vezanim karticama.
 */
@Getter
@Setter
public class AccountDetailsResponseDto {
    /** Naziv koji je korisnik dao za svoj račun. */
    private String nazivRacuna;

    /** Jedinstveni 18-cifreni broj računa. */
    private String brojRacuna;

    /** ID vlasnika računa (klijenta). */
    private Long vlasnik;

    /** Tip računa (tekuci / devizni) na srpskom. */
    private String tip;

    /** Iznos dostupan za trošenje (bez rezervisanih sredstava). */
    private BigDecimal raspolozivoStanje;

    /** Iznos koji je rezervisan / blokiram na računu. */
    private BigDecimal rezervisanaSredstva;

    /** Ukupno stanje računa (raspoloživo + rezervisano). */
    private BigDecimal stanjeRacuna;

    /** Naziv firme (ako je to poslovni račun). */
    private String nazivFirme;

    /** Kod valute (USD, EUR, RSD, itd.). */
    private String currency;

    /** Dnevni limit trošenja na računu. */
    private BigDecimal dailyLimit;

    /** Mesečni limit trošenja na računu. */
    private BigDecimal monthlyLimit;

    /** Iznos potošen u tekućem danu. */
    private BigDecimal dailySpending;

    /** Iznos potošen u tekućem mesecu. */
    private BigDecimal monthlySpending;

    /** Datum i vreme kreiranja računa. */
    private LocalDateTime creationDate;

    /** Datum isteka računa (ako je primenjiv). */
    private LocalDate expirationDate;

    /** Status računa (ACTIVE / INACTIVE). */
    private String status;

    /** Kategorija računa (CHECKING / FOREIGN_CURRENCY). */
    private String accountCategory;

    /** Tip vlasnistva računa (PERSONAL / BUSINESS). */
    private String accountType;

    /** Podtip računa (za tekuće račune, npr. STANDARDNI, ZA_MLADE). */
    private String subtype;

    /** Matični broj firme (ako je to poslovni račun). */
    private String companyRegistrationNumber;

    /** Poreski broj firme (ako je to poslovni račun). */
    private String companyTaxId;

    /** Šifra delatnosti firme (ako je to poslovni račun). */
    private String companyActivityCode;

    /** Adresa firme (ako je to poslovni račun). */
    private String companyAddress;

    /** ID vlasnika firme (ako je to poslovni račun). */
    private Long companyOwnerId;

    /** Lista kartice vezane za ovaj račun. */
    private List<CardResponseDto> cards = List.of();

    /**
     * Kreira DTO od {@link Account} entiteta.
     * <p>
     * Automatski mapira sve polje i odrđuje tip računa (tekuci / devizni)
     * na osnovu tipa entiteta.
     *
     * @param account entitet iz baze
     */
    public AccountDetailsResponseDto(Account account) {
        this.nazivRacuna = account.getNazivRacuna();
        this.brojRacuna = account.getBrojRacuna();
        this.vlasnik = account.getVlasnik();
        this.raspolozivoStanje = account.getRaspolozivoStanje();
        this.stanjeRacuna = account.getStanje();
        this.currency = account.getCurrency() != null ? account.getCurrency().getOznaka().name() : null;
        this.dailyLimit = account.getDnevniLimit();
        this.monthlyLimit = account.getMesecniLimit();
        this.dailySpending = account.getDnevnaPotrosnja();
        this.monthlySpending = account.getMesecnaPotrosnja();
        this.creationDate = account.getDatumIVremeKreiranja();
        this.expirationDate = account.getDatumIsteka();
        this.status = account.getStatus() != null ? account.getStatus().name() : null;

        if (account.getStanje() != null && account.getRaspolozivoStanje() != null) {
            this.rezervisanaSredstva = account.getStanje().subtract(account.getRaspolozivoStanje());
        }

        if (account instanceof CheckingAccount ca) {
            this.tip = "tekuci";
            this.accountCategory = "CHECKING";
            this.accountType = ca.getAccountConcrete().getAccountOwnershipType().name();
            this.subtype = ca.getAccountConcrete().name();
        } else if (account instanceof FxAccount fa) {
            this.tip = "devizni";
            this.accountCategory = "FOREIGN_CURRENCY";
            this.accountType = fa.getAccountOwnershipType().name();
            this.subtype = null;
        }

        Company company = account.getCompany();
        if (company != null) {
            this.nazivFirme = company.getNaziv();
            this.companyRegistrationNumber = company.getMaticni_broj();
            this.companyTaxId = company.getPoreski_broj();
            this.companyActivityCode = company.getSifraDelatnosti() != null ? company.getSifraDelatnosti().getSifra() : null;
            this.companyAddress = company.getAdresa();
            this.companyOwnerId = company.getVlasnik();
        }
    }
}
