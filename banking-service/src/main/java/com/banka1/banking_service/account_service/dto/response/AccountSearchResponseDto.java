package com.banka1.banking_service.account_service.dto.response;

import com.banka1.banking_service.account_service.domain.Account;
import com.banka1.banking_service.account_service.domain.CheckingAccount;
import com.banka1.banking_service.account_service.domain.FxAccount;
import com.banka1.banking_service.account_service.domain.enums.AccountOwnershipType;
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

    public AccountSearchResponseDto(Account account) {
        this.brojRacuna = account.getBrojRacuna();
        this.ime = account.getImeVlasnikaRacuna();
        this.prezime = account.getPrezimeVlasnikaRacuna();
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
