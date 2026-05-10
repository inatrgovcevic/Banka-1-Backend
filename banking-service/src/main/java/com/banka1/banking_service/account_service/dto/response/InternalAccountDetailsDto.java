package com.banka1.banking_service.account_service.dto.response;

import com.banka1.banking_service.account_service.domain.Account;

import java.math.BigDecimal;

/**
 * DTO za interne pozive izmedju servisa (npr. transfer-service, order-service).
 * Sadrzi osnovne informacije o racunu sa engleskim imenima polja
 * kako bi bili kompatibilni sa ocekivanim formatom koji koriste drugi servisi.
 *
 * <p>{@code id} je primarni kljuc racuna u bazi i koristi se kao Long identifikator
 * u inter-servisnoj komunikaciji (npr. order-service prosledjuje ovu vrednost
 * pri pozivu {@code /internal/accounts/id/{accountId}/details} ili kao
 * {@code accountId} u Order entitetu). Razlikovati od {@code accountNumber} koji
 * je 16-cifreni broj racuna (String), namenjen za matching u transakcijama
 * kroz {@code PaymentDto}.
 */
public record InternalAccountDetailsDto(
        Long id,
        String accountNumber,
        Long ownerId,
        String currency,
        BigDecimal availableBalance,
        String status,
        String accountType,
        String email,
        String username
) {
    public static InternalAccountDetailsDto from(Account account) {
        String accountType = null;
        if (account instanceof com.banka1.banking_service.account_service.domain.CheckingAccount ca) {
            accountType = ca.getAccountConcrete().getAccountOwnershipType().name();
        } else if (account instanceof com.banka1.banking_service.account_service.domain.FxAccount fa) {
            accountType = fa.getAccountOwnershipType().name();
        }
        return new InternalAccountDetailsDto(
                account.getId(),
                account.getBrojRacuna(),
                account.getVlasnik(),
                account.getCurrency() != null ? account.getCurrency().getOznaka().name() : null,
                account.getRaspolozivoStanje(),
                account.getStatus() != null ? account.getStatus().name() : null,
                accountType,
                account.getEmail(),
                account.getUsername()
        );
    }
}
