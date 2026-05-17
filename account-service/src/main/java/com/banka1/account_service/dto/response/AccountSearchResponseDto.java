package com.banka1.account_service.dto.response;

import com.banka1.account_service.domain.Account;
import com.banka1.account_service.domain.CheckingAccount;
import com.banka1.account_service.domain.FxAccount;
import com.banka1.account_service.domain.enums.AccountOwnershipType;
import com.banka1.account_service.domain.enums.Status;
import lombok.AllArgsConstructor;
import lombok.Getter;
import lombok.NoArgsConstructor;
import lombok.Setter;

@Getter
@Setter
@NoArgsConstructor
@AllArgsConstructor
public class AccountSearchResponseDto {
    private String brojRacuna;
    private String ime;
    private String prezime;
    private AccountOwnershipType accountOwnershipType;
    private String tekuciIliDevizni;
    private java.math.BigDecimal stanje;
    private java.math.BigDecimal raspolozivoStanje;
    private java.math.BigDecimal rezervisanaSredstva;
    private String currency;
    private Status status;
    private Boolean isSystemAccount;
    private Long vlasnik;
    private Long zaposlen;
    private java.math.BigDecimal dnevniLimit;
    private java.math.BigDecimal mesecniLimit;
    private java.math.BigDecimal dnevnaPotrosnja;
    private java.math.BigDecimal mesecnaPotrosnja;
    private java.time.LocalDate datumIsteka;

    public AccountSearchResponseDto(Account account) {
        this.brojRacuna = account.getBrojRacuna();
        this.ime = account.getImeVlasnikaRacuna();
        this.prezime = account.getPrezimeVlasnikaRacuna();
        this.stanje = account.getStanje();
        this.raspolozivoStanje = account.getRaspolozivoStanje();
        this.rezervisanaSredstva = account.getStanje() != null && account.getRaspolozivoStanje() != null
                ? account.getStanje().subtract(account.getRaspolozivoStanje())
                : java.math.BigDecimal.ZERO;
        this.currency = account.getCurrency() != null && account.getCurrency().getOznaka() != null
                ? account.getCurrency().getOznaka().name()
                : null;
        this.status = account.getStatus();
        this.isSystemAccount = account.getVlasnik() != null && account.getVlasnik() < 0;
        this.vlasnik = account.getVlasnik();
        this.zaposlen = account.getZaposlen();
        this.dnevniLimit = account.getDnevniLimit();
        this.mesecniLimit = account.getMesecniLimit();
        this.dnevnaPotrosnja = account.getDnevnaPotrosnja();
        this.mesecnaPotrosnja = account.getMesecnaPotrosnja();
        this.datumIsteka = account.getDatumIsteka();
        if (account instanceof CheckingAccount ca) {
            tekuciIliDevizni = "tekuci";
            if (ca.getAccountConcrete() != null) {
                accountOwnershipType = ca.getAccountConcrete().getAccountOwnershipType();
            } else {
                // Fallback for legacy rows where account_concrete is unexpectedly null.
                accountOwnershipType = ca.getCompany() == null
                        ? AccountOwnershipType.PERSONAL
                        : AccountOwnershipType.BUSINESS;
            }
        } else if (account instanceof FxAccount fa) {
            tekuciIliDevizni = "devizni";
            accountOwnershipType = fa.getAccountOwnershipType();
        }
    }
}
