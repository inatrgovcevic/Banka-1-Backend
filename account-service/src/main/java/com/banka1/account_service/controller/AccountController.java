package com.banka1.account_service.controller;

import com.banka1.account_service.domain.enums.CurrencyCode;
import com.banka1.account_service.dto.request.BankPaymentDto;
import com.banka1.account_service.dto.request.CreditDebitAccountDto;
import com.banka1.account_service.dto.request.CreditDebitBankDto;
import com.banka1.account_service.dto.request.OneSidedTransactionDto;
import com.banka1.account_service.dto.request.PaymentDto;
import com.banka1.account_service.dto.response.InfoResponseDto;
import com.banka1.account_service.dto.response.InternalAccountDetailsDto;
import com.banka1.account_service.dto.response.UpdatedBalanceResponseDto;
import com.banka1.account_service.service.AccountService;
import jakarta.validation.Valid;
import lombok.AllArgsConstructor;
import org.springframework.http.HttpStatus;
import org.springframework.http.ResponseEntity;
import org.springframework.security.access.prepost.PreAuthorize;
import org.springframework.security.core.annotation.AuthenticationPrincipal;
import org.springframework.security.oauth2.jwt.Jwt;
import org.springframework.web.bind.annotation.*;

/**
 * REST kontroler za interne operacije nad racunima u Banka1 sistemu.
 * <p>
 * Ovi endpointi su namenjeni samo inter-servisnoj komunikaciji (npr. od transfer servisa
 * ili transaction servisa) i zahtevaju SERVICE ulogu za pristup.
 * <p>
 * Omogucava:
 * <ul>
 *   <li>Obradu financijskih transakcija (transfer novca, transakcije)</li>
 *   <li>Preuzimanje informacija o racunima radi verifikacije</li>
 * </ul>
 */
@RestController
@AllArgsConstructor
@RequestMapping("/internal/accounts")
@PreAuthorize("hasRole('SERVICE')")
public class AccountController {

    /** Servis za izvrsavanje transakcija i transfera. */
    private AccountService accountService;

    /**
     * Obradi financijsku transakciju na racunu sa debit/credit operacijom.
     * <p>
     * Transakcija se vrsi preko PaymentDto koji sadrzi sve potrebne informacije
     * (brojevi racuna, iznos, provizija, ID klijenta, itd.). Validira oba racuna
     * i njihove banka-racune pre nego sto prenese sredstva. Proverava da racuni
     * pripadaju razlicitim vlasnicima.
     *
     * @param jwt JWT token servisa koji prave zahtev
     * @param paymentDto podaci o transakciji (broj racuna posiljaoca, primaoca, iznos, provizija)
     * @return {@link UpdatedBalanceResponseDto} sa azuriranim stanjem oba racuna
     * @throws IllegalArgumentException ako racun ne postoji, nije aktivan, vlasnik je drugaciji ili client ID nedostaje
     * @throws IllegalStateException ako banka-racun ne postoji ili nije aktivan
     */
    @PostMapping("/transaction")
    public ResponseEntity<UpdatedBalanceResponseDto> transaction(@AuthenticationPrincipal Jwt jwt, @RequestBody @Valid PaymentDto paymentDto) {
        return new ResponseEntity<>(accountService.transaction(paymentDto),HttpStatus.OK);
    }

    /**
     * GHI #199: jednostrani debit za trade-leg klijentskog BUY-a.
     * Bankin racun se NE dira (po PM direktivi); samo se sredstva skidaju sa
     * korisnikovog racuna.
     */
    @PostMapping("/exchange/buy")
    public ResponseEntity<UpdatedBalanceResponseDto> exchangeBuy(@AuthenticationPrincipal Jwt jwt, @RequestBody @Valid OneSidedTransactionDto request) {
        return new ResponseEntity<>(accountService.exchangeBuy(request), HttpStatus.OK);
    }

    /**
     * GHI #199: jednostrani credit za trade-leg klijentskog SELL-a.
     * Bankin racun se NE dira na drugoj strani; korisnikov racun se direktno
     * popunjava trade proceeds-om.
     */
    @PostMapping("/exchange/sell")
    public ResponseEntity<UpdatedBalanceResponseDto> exchangeSell(@AuthenticationPrincipal Jwt jwt, @RequestBody @Valid OneSidedTransactionDto request) {
        return new ResponseEntity<>(accountService.exchangeSell(request), HttpStatus.OK);
    }


    @PostMapping("/transactionFromBank")
    public ResponseEntity<Void> transactionFromBank(@AuthenticationPrincipal Jwt jwt, @RequestBody @Valid BankPaymentDto bankPaymentDto) {
        accountService.transactionFromBank(bankPaymentDto);
        return new ResponseEntity<>(HttpStatus.OK);
    }

    @PostMapping("/credit")
    public ResponseEntity<Void> credit(@AuthenticationPrincipal Jwt jwt, @RequestBody @Valid CreditDebitAccountDto creditDebitAccountDto)
    {
        accountService.creditAccount(creditDebitAccountDto);
        return new ResponseEntity<>(HttpStatus.OK);
    }

    @PostMapping("/creditBank")
    public ResponseEntity<Void> creditBank(@AuthenticationPrincipal Jwt jwt, @RequestBody @Valid CreditDebitBankDto creditDebitBankDto)
    {
        accountService.creditBank(creditDebitBankDto);
        return new ResponseEntity<>(HttpStatus.OK);
    }

    @PostMapping("/debit")
    public ResponseEntity<Void> debit(@AuthenticationPrincipal Jwt jwt, @RequestBody @Valid CreditDebitAccountDto creditDebitAccountDto)
    {
        accountService.debitAccount(creditDebitAccountDto);
        return new ResponseEntity<>(HttpStatus.OK);
    }

    @PostMapping("/debitBank")
    public ResponseEntity<Void> debitBank(@AuthenticationPrincipal Jwt jwt, @RequestBody @Valid CreditDebitBankDto creditDebitBankDto)
    {
        accountService.debitBank(creditDebitBankDto);
        return new ResponseEntity<>(HttpStatus.OK);
    }


    @GetMapping("/id/{accountId}/details")
    public ResponseEntity<InternalAccountDetailsDto> getAccountDetailsById(@AuthenticationPrincipal Jwt jwt, @PathVariable Long accountId) {
        return new ResponseEntity<>(accountService.getAccountDetails(accountId), HttpStatus.OK);
    }

    @GetMapping("/bank/{currencyCode}")
    public ResponseEntity<InternalAccountDetailsDto> getBankAccountDetails(@AuthenticationPrincipal Jwt jwt, @PathVariable CurrencyCode currencyCode) {
        return new ResponseEntity<>(accountService.getBankAccountDetails(currencyCode), HttpStatus.OK);
    }

    /**
     * Vraca drzavni racun za zadatu valutu. Drzava je modelovana kao zasebna
     * firma sa vlasnikom {@code -2}; u nasem sistemu ima samo RSD racun koji se
     * koristi za naplatu poreza na kapitalnu dobit i za namirenje opcionih ugovora
     * (exercise). Ovaj endpoint se koristi od strane order-service-a umesto
     * ranijeg {@code /bank/{currencyCode}} poziva, kako bi se drzavna sredstva
     * odvojila od bankinog kapitala.
     *
     * @param jwt JWT token servisa koji pravi zahtev
     * @param currencyCode kod valute (u praksi samo RSD)
     * @return {@link InternalAccountDetailsDto} sa detaljima drzavnog racuna
     */
    @GetMapping("/state/{currencyCode}")
    public ResponseEntity<InternalAccountDetailsDto> getStateAccountDetails(@AuthenticationPrincipal Jwt jwt, @PathVariable CurrencyCode currencyCode) {
        if (currencyCode != CurrencyCode.RSD) {
            throw new IllegalArgumentException(
                    "Drzavni racun postoji samo za RSD valutu, trazena valuta: " + currencyCode);
        }
        return new ResponseEntity<>(accountService.getStateAccountDetails(currencyCode), HttpStatus.OK);
    }

    /**
     * Obradi transfer novca izmedju dva racuna istog vlasnika.
     * <p>
     * Razlikuje se od {@code /transaction} jer zahteva da oba racuna
     * pripadaju istom vlasniku. Azurira stanja oba racuna (izvor i odrediste)
     * kao i banka-racune za komisije.
     *
     * @param jwt JWT token servisa koji prave zahtev
     * @param paymentDto podaci o transferu (brojevi racuna, iznos, provizija, ID klijenta)
     * @return {@link UpdatedBalanceResponseDto} sa azuriranim stanjem oba racuna
     * @throws IllegalArgumentException ako racuni ne pripadaju istom vlasniku ili ne postoje
     * @throws IllegalStateException ako banka-racun nije aktivan
     */
    @PostMapping("/transfer")
    public ResponseEntity<UpdatedBalanceResponseDto> transfer(@AuthenticationPrincipal Jwt jwt, @RequestBody @Valid PaymentDto paymentDto) {
        return new ResponseEntity<>(accountService.transfer(paymentDto),HttpStatus.OK);
    }

    /**
     * Vraca detalje racuna po broju racuna.
     * Koristi se od strane transfer-service-a za proveru vlasnika i valute pre izvrsavanja transfera.
     * Polja su u engleskom formatu kako bi bila kompatibilna sa AccountDto u transfer-service-u.
     *
     * @param jwt JWT token servisa koji prave zahtev
     * @param accountNumber broj racuna
     * @return {@link InternalAccountDetailsDto} sa detaljima racuna
     * @throws IllegalArgumentException ako racun ne postoji
     */
    @GetMapping("/{accountNumber}/details")
    public ResponseEntity<InternalAccountDetailsDto> getAccountDetails(@AuthenticationPrincipal Jwt jwt, @PathVariable String accountNumber) {
        return new ResponseEntity<>(accountService.getAccountDetails(accountNumber), HttpStatus.OK);
    }

    /**
     * Preuzima informacije o dva racuna za verifikaciju pre transakcije.
     * <p>
     * Koristi se od strane drugih servisa (npr. transaction-service) da provjeri
     * validnost racuna, njihove valute i ID-eve vlasnika pre izvrsavanja transakcije.
     *
     * @param jwt JWT token servisa koji prave zahtev
     * @param fromBankNumber broj izvornog racuna posiljaoca
     * @param toBankNumber broj odredisnog racuna primaoca
     * @return {@link InfoResponseDto} sa valutama, ID-evima vlasnika i kontakt informacijama
     * @throws IllegalArgumentException ako neki od racuna ne postoji ili nije aktivan
     */
    @GetMapping("/info")
    public ResponseEntity<InfoResponseDto> info(@AuthenticationPrincipal Jwt jwt,@RequestParam String fromBankNumber,@RequestParam String toBankNumber)
    {
        return new ResponseEntity<>(accountService.info(jwt,fromBankNumber,toBankNumber),HttpStatus.OK);
    }

    /**
     * PR_14 C14.8: kreira sistemski racun sa unapred zadatim brojem racuna.
     *
     * <p>Trading-service zove ovo prilikom kreiranja investicionog fonda — fond
     * mora imati pravi {@code Account} red kako bi SAGA invest/redeem mogli da
     * skidaju i dodaju novac na racun fonda.
     */
    @PreAuthorize("hasAnyRole('SERVICE','BASIC')")
    @PostMapping("/system")
    public ResponseEntity<InternalAccountDetailsDto> createSystemAccount(
            @AuthenticationPrincipal Jwt jwt,
            @RequestBody @Valid com.banka1.account_service.dto.request.CreateSystemAccountDto dto) {
        return new ResponseEntity<>(accountService.createSystemAccount(dto), HttpStatus.CREATED);
    }
}
