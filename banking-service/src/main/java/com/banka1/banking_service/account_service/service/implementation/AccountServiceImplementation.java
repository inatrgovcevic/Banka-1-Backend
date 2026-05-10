package com.banka1.banking_service.account_service.service.implementation;

import com.banka1.banking_service.account_service.domain.Account;
import com.banka1.banking_service.account_service.domain.Currency;
import com.banka1.banking_service.account_service.domain.enums.CurrencyCode;
import com.banka1.banking_service.account_service.domain.enums.Status;
import com.banka1.banking_service.account_service.dto.request.BankPaymentDto;
import com.banka1.banking_service.account_service.dto.request.CreditDebitAccountDto;
import com.banka1.banking_service.account_service.dto.request.CreditDebitBankDto;
import com.banka1.banking_service.account_service.dto.request.PaymentDto;
import com.banka1.banking_service.account_service.dto.response.InfoResponseDto;
import com.banka1.banking_service.account_service.dto.response.InternalAccountDetailsDto;
import com.banka1.banking_service.account_service.dto.response.UpdatedBalanceResponseDto;
import com.banka1.banking_service.account_service.repository.AccountRepository;
import com.banka1.banking_service.account_service.repository.CurrencyRepository;
import com.banka1.banking_service.account_service.service.AccountService;
import com.banka1.banking_service.account_service.service.TransactionalService;
import jakarta.persistence.OptimisticLockException;
import lombok.RequiredArgsConstructor;
import org.springframework.orm.ObjectOptimisticLockingFailureException;
import org.springframework.security.oauth2.jwt.Jwt;
import org.springframework.stereotype.Service;

import java.time.LocalDate;
import java.util.NoSuchElementException;

/**
 * Implementacija servisa za izvrsavanje internih transakcija i transfera.
 * <p>
 * Validira račune, njihove statusu i saldone, zatim delegira atomičnu
 * operaciju transfera na {@link TransactionalService}. Koristi retry logiku
 * za optimističke lock greške.
 */
@RequiredArgsConstructor
@Service
public class AccountServiceImplementation implements AccountService {
    /** Servis za atomične debitne/kreditne operacije. */
    private final TransactionalService transactionalService;
    /** Repozitorijum za pristup računima iz baze. */
    private final AccountRepository accountRepository;

    private final CurrencyRepository currencyRepository;

    /**
     * Validira da račun postoji, ima ACTIVE status i nije istekao.
     *
     * @param accountNumber broj računa koji se validira
     * @return validiran Account objekat
     * @throws IllegalArgumentException ako račun ne postoji, nije aktivan ili je istekao
     */
    private Account validate(String accountNumber)
    {
        Account account = accountRepository.findByBrojRacuna(accountNumber).orElse(null);
        if(account==null)
            throw new IllegalArgumentException("Ne postoji racun:"+accountNumber);
        if(account.getStatus()== Status.INACTIVE)
            throw new IllegalArgumentException("Racun je neaktivan:"+accountNumber);
        if(account.getDatumIsteka()!=null&&account.getDatumIsteka().isBefore(LocalDate.now()))
            throw new IllegalArgumentException("Racun je istekao:"+accountNumber);
        return account;
    }
    /**
     * Pronalazi i validira banka-račun u tražnoj valuti.
     * <p>
     * Banka-računi se identifikuju po vlasnikuID=-1L i valuti računa.
     *
     * @param to račun čija valuta se koristi za pronalaženje odgovarajućeg banka-računa
     * @return validiran Account banka-račun
     * @throws IllegalStateException ako banka-račun ne postoji, nije aktivan ili je istekao
     */
    private Account validateBank(Account to)
    {
        Account account=accountRepository.findByVlasnikAndCurrency(-1L,to.getCurrency()).orElse(null);
        if(account==null)
            throw new IllegalStateException("Greska u sistemu fali banka");
        if(account.getStatus()== Status.INACTIVE)
            throw new IllegalStateException("Racun banke je neaktivan");
        if(account.getDatumIsteka()!=null&&account.getDatumIsteka().isBefore(LocalDate.now()))
            throw new IllegalStateException("Racun banke je istekao");
        return account;
    }

    private Account validateBank(CurrencyCode currencyCode)
    {
        Currency currency=currencyRepository.findByOznaka(currencyCode).orElse(null);
        if(currency==null)
            throw new IllegalArgumentException("Ne postoji ovaj currency "+currencyCode);
        Account account=accountRepository.findByVlasnikAndCurrency(-1L,currency).orElse(null);
        if(account==null)
            throw new IllegalStateException("Greska u sistemu fali banka");
        if(account.getStatus()== Status.INACTIVE)
            throw new IllegalStateException("Racun banke je neaktivan");
        if(account.getDatumIsteka()!=null&&account.getDatumIsteka().isBefore(LocalDate.now()))
            throw new IllegalStateException("Racun banke je istekao");
        return account;
    }

    /**
     * Izvršava transfer sa retry logikom za optimističke lock greške.
     * <p>
     * Prvo validira vlasnika računa, zatim pokušava transfer sa do 3 pokušaja
     * u slučaju konkurentnog pristupa istom računu.
     *
     * @param paymentDto podaci o transakciji
     * @param from izvorni račun
     * @param to odredišni račun
     * @param bankSender banka-račun posiljaoca
     * @param bankTarget banka-račun primaoca
     * @return azurirani saldoi nakon transfera
     * @throws IllegalArgumentException ako korisnik nije vlasnik računa
     * @throws ObjectOptimisticLockingFailureException ako se greška ponovi 3 puta
     */
    private UpdatedBalanceResponseDto execute(PaymentDto paymentDto, Account from, Account to, Account bankSender, Account bankTarget) {
        if(!from.getVlasnik().equals(paymentDto.getClientId()))
            throw new IllegalArgumentException("Nisi vlasnik racuna");
        for(int i = 0; true; i++) {
            try {
                return transactionalService.transfer(from,to,bankSender,bankTarget,paymentDto);
            } catch (ObjectOptimisticLockingFailureException | OptimisticLockException optimisticLockException) {
                if(i>=2)
                    throw optimisticLockException;
            }
        }
    }

    @Override
    public void creditBank(CreditDebitBankDto creditDebitBankDto) {
        CurrencyCode currencyCode;
        try
        {
            currencyCode=CurrencyCode.valueOf(creditDebitBankDto.getCurrencyCode().toUpperCase());
        }
        catch (Exception e)
        {
            throw new IllegalArgumentException("Ne postoji ovaj currencyCode: "+ creditDebitBankDto.getCurrencyCode());
        }
        Account account=validateBank(currencyCode);
        for(int i = 0; true; i++) {
            try {
                transactionalService.creditTransactional(account, creditDebitBankDto.getAmount());
                return;
            } catch (ObjectOptimisticLockingFailureException | OptimisticLockException optimisticLockException) {
                if(i>=2)
                    throw optimisticLockException;
            }
        }
    }

    @Override
    public void creditAccount(CreditDebitAccountDto creditDebitAccountDto) {
        Account account=validate(creditDebitAccountDto.getAccountNumber());
        if(!account.getVlasnik().equals(-1L) && !account.getVlasnik().equals(creditDebitAccountDto.getClientId()))
            throw new IllegalArgumentException("Nisi vlasnik racuna");
        for(int i = 0; true; i++) {
            try {
                transactionalService.creditTransactional(account, creditDebitAccountDto.getAmount());
                return;
            } catch (ObjectOptimisticLockingFailureException | OptimisticLockException optimisticLockException) {
                if(i>=2)
                    throw optimisticLockException;
            }
        }
    }

    @Override
    public void debitBank(CreditDebitBankDto creditDebitBankDto) {
        CurrencyCode currencyCode;
        try
        {
            currencyCode=CurrencyCode.valueOf(creditDebitBankDto.getCurrencyCode().toUpperCase());
        }
        catch (Exception e)
        {
            throw new IllegalArgumentException("Ne postoji ovaj currencyCode: "+ creditDebitBankDto.getCurrencyCode());
        }
        Account account=validateBank(currencyCode);
        for(int i = 0; true; i++) {
            try {
                transactionalService.debitTransactional(account, creditDebitBankDto.getAmount());
                return;
            } catch (ObjectOptimisticLockingFailureException | OptimisticLockException optimisticLockException) {
                if(i>=2)
                    throw optimisticLockException;
            }
        }
    }

    @Override
    public void debitAccount(CreditDebitAccountDto creditDebitAccountDto) {
        Account account=validate(creditDebitAccountDto.getAccountNumber());
        if(!account.getVlasnik().equals(-1L) && !account.getVlasnik().equals(creditDebitAccountDto.getClientId()))
            throw new IllegalArgumentException("Nisi vlasnik racuna");
        for(int i = 0; true; i++) {
            try {
                transactionalService.debitTransactional(account, creditDebitAccountDto.getAmount());
                return;
            } catch (ObjectOptimisticLockingFailureException | OptimisticLockException optimisticLockException) {
                if(i>=2)
                    throw optimisticLockException;
            }
        }
    }

    @Override
    public UpdatedBalanceResponseDto transaction(PaymentDto paymentDto) {
        Account from=validate(paymentDto.getFromAccountNumber());
        Account to=validate(paymentDto.getToAccountNumber());
        Account bankSender=validateBank(from);
        Account bankTarget=validateBank(to);
        if(paymentDto.getClientId()==null)
            throw new IllegalArgumentException("Unesi id clienta");
        if(from.getVlasnik().equals(to.getVlasnik()))
            throw new IllegalArgumentException("Tranzakcija se ne moze odvijati za racune istog vlasnike");
        return execute(paymentDto, from, to, bankSender, bankTarget);
    }

    @Override
    public void transactionFromBank(BankPaymentDto paymentDto) {
        if(paymentDto.getFromAccountNumber()==null && paymentDto.getToAccountNumber()==null)
            throw new IllegalArgumentException("Los unos");
        Account sender;
        Account recipient;
        if(paymentDto.getFromAccountNumber()==null) {
            recipient = validate(paymentDto.getToAccountNumber());
            sender = validateBank(recipient);
        }
        else
        {
            sender = validate(paymentDto.getFromAccountNumber());
            recipient = validateBank(sender);
        }
        for(int i = 0; true; i++) {
            try {
                transactionalService.transfer(sender,recipient,paymentDto);
                return;
            } catch (ObjectOptimisticLockingFailureException | OptimisticLockException optimisticLockException) {
                if(i>=2)
                    throw optimisticLockException;
            }
        }
    }


    @Override
    public UpdatedBalanceResponseDto transfer(PaymentDto paymentDto) {
        Account from=validate(paymentDto.getFromAccountNumber());
        Account to=validate(paymentDto.getToAccountNumber());
        Account bankSender=validateBank(from);
        Account bankTarget=validateBank(to);
        if(paymentDto.getClientId()==null)
            throw new IllegalArgumentException("Unesi id clienta");
        if(!from.getVlasnik().equals(to.getVlasnik()))
            throw new IllegalArgumentException("Transfer se moze odvijati samo za racune istog vlasnika");
        return execute(paymentDto, from, to, bankSender, bankTarget);
    }

    @Override
    public InternalAccountDetailsDto getAccountDetails(String accountNumber) {
        Account account = accountRepository.findByBrojRacuna(accountNumber).orElse(null);
        if (account == null)
            throw new NoSuchElementException("Ne postoji racun:" + accountNumber);
        return InternalAccountDetailsDto.from(account);
    }

    @Override
    public InternalAccountDetailsDto getAccountDetails(Long accountId) {
        Account account = accountRepository.findById(accountId).orElse(null);
        if (account == null)
            throw new NoSuchElementException("Ne postoji racun id:" + accountId);
        return InternalAccountDetailsDto.from(account);
    }

    @Override
    public InternalAccountDetailsDto getBankAccountDetails(CurrencyCode currencyCode) {
        Account account = accountRepository.findBankAccountByCurrencyCode(currencyCode).orElse(null);
        if (account == null)
            throw new NoSuchElementException("Ne postoji interni bankovni racun za valutu:" + currencyCode);
        return InternalAccountDetailsDto.from(account);
    }

    @Override
    public InternalAccountDetailsDto getStateAccountDetails(CurrencyCode currencyCode) {
        Account account = accountRepository.findStateAccountByCurrencyCode(currencyCode).orElse(null);
        if (account == null)
            throw new NoSuchElementException("Ne postoji drzavni racun za valutu:" + currencyCode);
        return InternalAccountDetailsDto.from(account);
    }

    @Override
    public InfoResponseDto info(Jwt jwt, String fromAccountNumber, String toAccountNumber) {
        Account fromAccount=accountRepository.findByBrojRacuna(fromAccountNumber).orElse(null);
        if(fromAccount==null)
            throw new IllegalArgumentException("Ne postoji from racun");
        if(fromAccount.getStatus()==Status.INACTIVE)
            throw new IllegalArgumentException("FromAccount nije aktivan");
        Account toAccount=accountRepository.findByBrojRacuna(toAccountNumber).orElse(null);
        if(toAccount==null)
            throw new IllegalArgumentException("Ne postoji to racun");
        if(toAccount.getStatus()==Status.INACTIVE)
            throw new IllegalArgumentException("ToAccount nije aktivan");
        return new InfoResponseDto(fromAccount.getCurrency().getOznaka(), toAccount.getCurrency().getOznaka(), fromAccount.getVlasnik(), toAccount.getVlasnik(),fromAccount.getEmail(),fromAccount.getUsername());

    }
}
