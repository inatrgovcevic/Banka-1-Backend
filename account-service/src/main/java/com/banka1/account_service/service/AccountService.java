package com.banka1.account_service.service;

import com.banka1.account_service.dto.request.BankPaymentDto;
import com.banka1.account_service.dto.request.CreditDebitAccountDto;
import com.banka1.account_service.dto.request.CreditDebitBankDto;
import com.banka1.account_service.dto.request.PaymentDto;

import com.banka1.account_service.domain.enums.CurrencyCode;
import com.banka1.account_service.dto.response.InfoResponseDto;
import com.banka1.account_service.dto.response.InternalAccountDetailsDto;
import com.banka1.account_service.dto.response.UpdatedBalanceResponseDto;
import org.springframework.security.oauth2.jwt.Jwt;

/**
 * Servis za izvrsavanje internih transakcija i transfera izmedju racuna.
 * Pozivaju ga interni servisi putem {@code /internal/accounts} endpointa.
 */
public interface AccountService {


    void creditBank(CreditDebitBankDto creditDebitBankDto);

    void creditAccount(CreditDebitAccountDto creditDebitAccountDto);

    void debitBank(CreditDebitBankDto creditDebitBankDto);

    void debitAccount(CreditDebitAccountDto creditDebitAccountDto);


    /**
     * Izvrsava transakciju izmedju racuna razlicitih vlasnika.
     * Validira oba racuna i banka-racune za svaku valutu, zatim prenosi sredstva.
     *
     * @param paymentDto podaci o placanju (brojevi racuna, iznosi, provizija, ID klijenta)
     * @return azurirana stanja oba racuna nakon transakcije
     */




    UpdatedBalanceResponseDto transaction(PaymentDto paymentDto);


    void transactionFromBank(BankPaymentDto paymentDto);

    /**
     * Jednostrana debit operacija za trade-leg klijentskog BUY-a (GHI #199).
     * Bankin racun se NE dira - po PM direktivi
     * <em>,,NE DAJE BANCI PARE, samo se skidaju sa racuna''</em>.
     *
     * @param request payload sa account identifikatorom + iznosom
     * @return azurirano stanje racuna nakon debita
     */
    UpdatedBalanceResponseDto exchangeBuy(com.banka1.account_service.dto.request.OneSidedTransactionDto request);

    /**
     * Jednostrana credit operacija za trade-leg klijentskog SELL-a (GHI #199).
     * Smer je obrnut od {@link #exchangeBuy}: korisniku se dodaju trade proceeds,
     * dok bankin racun ostaje netaknut.
     *
     * @param request payload sa account identifikatorom + iznosom
     * @return azurirano stanje racuna nakon credita
     */
    UpdatedBalanceResponseDto exchangeSell(com.banka1.account_service.dto.request.OneSidedTransactionDto request);

    /**
     * Izvrsava transfer izmedju dva racuna istog vlasnika.
     * Razlikuje se od {@link #transaction} po tome sto proverava da oba racuna
     * pripadaju istom vlasniku.
     *
     * @param paymentDto podaci o transferu (brojevi racuna, iznosi, provizija, ID klijenta)
     * @return azurirana stanja oba racuna nakon transfera
     */
    UpdatedBalanceResponseDto transfer(PaymentDto paymentDto);

    /**
     * Vraca informacije o valutama i vlasnicima dva racuna.
     * Koristi se od strane transaction-service-a za proveru pre izvrsavanja transakcije.
     *
     * @param jwt JWT token pozivaoca
     * @param fromAccountNumber broj racuna posiljaoca
     * @param toAccountNumber broj racuna primaoca
     * @return DTO sa valutama i ID-evima vlasnika oba racuna
     */
    InfoResponseDto info(Jwt jwt, String fromAccountNumber, String toAccountNumber);

    /**
     * Vraca detalje racuna po broju racuna.
     * Koristi se od strane transfer-service-a za proveru vlasnika i valute pre izvrsavanja transfera.
     *
     * @param accountNumber broj racuna
     * @return DTO sa detaljima racuna
     */
    InternalAccountDetailsDto getAccountDetails(String accountNumber);

    /**
     * Vraca detalje racuna po internom ID-u.
     *
     * @param accountId identifikator racuna
     * @return DTO sa detaljima racuna
     */
    InternalAccountDetailsDto getAccountDetails(Long accountId);

    /**
     * Vraca interni bankovni/drzavni racun za zadatu valutu.
     *
     * @param currencyCode kod valute
     * @return DTO sa detaljima internog racuna
     */
    InternalAccountDetailsDto getBankAccountDetails(CurrencyCode currencyCode);

    /**
     * Vraca drzavni (State) racun za zadatu valutu. Drzava je modelovana kao zasebna
     * firma sa vlasnikom {@code -2} i u nasem sistemu se koristi za naplatu poreza
     * na kapitalnu dobit i za namirenje opcionih ugovora (exercise). U praksi,
     * state ima samo RSD racun.
     *
     * @param currencyCode kod valute (u praksi samo RSD)
     * @return DTO sa detaljima drzavnog racuna
     */
    InternalAccountDetailsDto getStateAccountDetails(CurrencyCode currencyCode);

    /**
     * Kreira sistemski (company-owned) racun sa unapred zadatim brojem racuna.
     * Koristi se kad trading-service kreira investicioni fond i potreban mu je
     * pravi {@code Account} red kako bi SAGA invest/redeem mogli da skidaju i
     * dodaju novac na racun fonda (PR_14 C14.8).
     *
     * <p>Razlikuje se od {@link #createCheckingAccount(Jwt, com.banka1.account_service.dto.request.CheckingDto)}
     * po tome sto preskace lookup klijenta po JMBG-u i kreira "system" racun
     * sa specijalnim {@code ownerId}-jem (npr. {@code -F<fundId>} dogovoreno).
     *
     * @return DTO sa detaljima novog racuna
     */
    InternalAccountDetailsDto createSystemAccount(com.banka1.account_service.dto.request.CreateSystemAccountDto dto);
}
