package com.banka1.account_service.service.implementation;

import com.banka1.account_service.domain.Account;
import com.banka1.account_service.domain.SystemAccountIds;
import com.banka1.account_service.dto.request.BankPaymentDto;
import com.banka1.account_service.dto.request.PaymentDto;
import com.banka1.account_service.dto.response.UpdatedBalanceResponseDto;
import com.banka1.account_service.exception.BusinessException;
import com.banka1.account_service.exception.ErrorCode;
import com.banka1.account_service.repository.AccountRepository;
import com.banka1.account_service.service.TransactionalService;
import lombok.RequiredArgsConstructor;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;

import java.math.BigDecimal;

/**
 * Implementacija servisa za atomične debitne/kreditne operacije nad računima.
 * <p>
 * Sve operacije su transakcijske i validiraju saldoe i dnevne/mesečne limite
 * pre nego što se izmene izvršavaju na bazi podataka.
 */
@Service
@RequiredArgsConstructor
public class TransactionalServiceImplementation implements TransactionalService {
    /** Repozitorijum za pristup računima iz baze. */
    private final AccountRepository accountRepository;

    /**
     * Oduzima sredstva sa računa (debit operacija).
     * <p>
     * Validira iznos, dostupan saldo, dnevni i mesečni limit pre nego što
     * se ažurira stanje iLimit potrošnje.
     *
     * @param account račun sa kojeg se oduzimaju sredstva
     * @param amount iznos koji se oduzima
     * @throws IllegalArgumentException ako je iznos <= 0
     * @throws BusinessException ako nema dovoljno sredstava ili je limit prekoračen
     */
    public void debit(Account account, BigDecimal amount) {
        if (amount == null || amount.compareTo(BigDecimal.ZERO) <= 0)
            throw new IllegalArgumentException("Iznos mora biti veci od 0");
        if(account.getRaspolozivoStanje().compareTo(amount)<0)
            throw new BusinessException(ErrorCode.INSUFFICIENT_FUNDS,ErrorCode.INSUFFICIENT_FUNDS.getTitle());
        if(account.getDnevniLimit() != null && account.getDnevnaPotrosnja().add(amount).compareTo(account.getDnevniLimit())>0)
            throw new BusinessException(ErrorCode.DAILY_LIMIT_EXCEEDED,ErrorCode.DAILY_LIMIT_EXCEEDED.getTitle());
        if(account.getMesecniLimit() != null && account.getMesecnaPotrosnja().add(amount).compareTo(account.getMesecniLimit())>0)
            throw new BusinessException(ErrorCode.MONTHLY_LIMIT_EXCEEDED,ErrorCode.MONTHLY_LIMIT_EXCEEDED.getTitle());
        account.setStanje(account.getStanje().subtract(amount));
        account.setRaspolozivoStanje(account.getRaspolozivoStanje().subtract(amount));
        account.setDnevnaPotrosnja(account.getDnevnaPotrosnja().add(amount));
        account.setMesecnaPotrosnja(account.getMesecnaPotrosnja().add(amount));
        accountRepository.save(account);
    }
    /**
     * Dodaje sredstva na račun (credit operacija).
     * <p>
     * Validira iznos pre nego što se ažurira stanje računa.
     *
     * @param account račun na koji se dodaju sredstva
     * @param amount iznos koji se dodaje
     * @throws IllegalArgumentException ako je iznos <= 0
     */
    public void credit(Account account, BigDecimal amount) {
        if (amount == null || amount.compareTo(BigDecimal.ZERO) <= 0)
            throw new IllegalArgumentException("Iznos mora biti veci od 0");
        account.setStanje(account.getStanje().add(amount));
        account.setRaspolozivoStanje(account.getRaspolozivoStanje().add(amount));
        accountRepository.save(account);
    }

    /**
     * Atomičan transfer sredstava između dva računa sa različitim scenarima.
     * <p>
     * Scenario 1 - Ista valuta: Posiljaoc se debita za iznos+komisiju, primaoc se kreditira za iznos,
     * banka se kreditira za komisiju.
     * <p>
     * Scenario 2 - Različite valute: Posiljaoc se debita za iznos (u svojoj valuti), njegov banka-račun
     * se kreditira. Primaočev banka-račun se debita za iznos (u zavisnosti od kursa), primaoc se kreditira
     * za iznos minus komisija.
     *
     * @param from izvorni račun posiljaoca
     * @param to odredišni račun primaoca
     * @param bankSender banka-račun u valuti posiljaoca
     * @param bankTarget banka-račun u valuti primaoca
     * @param paymentDto podaci o plaćanju (iznosi, komisija)
     * @return azurirani saldoi oba klijentska računa
     * @throws BusinessException ako nema dovoljno sredstava ili je limit prekoračen
     */
    @Transactional
    @Override
    public UpdatedBalanceResponseDto transfer(Account from, Account to, Account bankSender, Account bankTarget, PaymentDto paymentDto) {
        BigDecimal commission = paymentDto.getCommission() != null ? paymentDto.getCommission() : BigDecimal.ZERO;
        if (from.getCurrency().getOznaka() == to.getCurrency().getOznaka()) {
            // Spec Celina 2 ("Provizije transakcija se prebacuju na racun banke u toj valuti"):
            // ako je provizija > 0 za istu valutu (npr. cross-client placanje), ona ide na
            // bankin racun u toj valuti — ne sme nestati.
            debit(from, paymentDto.getFromAmount().add(commission));
            credit(to, paymentDto.getToAmount());
            if (commission.signum() > 0 && bankSender != null) {
                credit(bankSender, commission);
            }
        } else {
            if (to.getVlasnik().equals(SystemAccountIds.BANK)) {
                debit(from, paymentDto.getFromAmount());
                credit(to, paymentDto.getToAmount());
                return new UpdatedBalanceResponseDto(from.getStanje(), to.getStanje());
            }
            // Cross-currency: spec Celina 2 ("Primer 2") trazi rutiranje preko RSD bank racuna
            // sa proviziom na svakom koraku. Ovde imamo dva-leg model: from -> bankSender (ista
            // valuta) -> bankTarget (target valuta) -> to. Ako je jedan od bank racuna RSD,
            // efekat je isti kao spec Primer 2. Provizija se naplacuje na primaocevoj strani
            // i ostaje na bankTarget racunu.
            debit(from, paymentDto.getFromAmount());
            credit(bankSender, paymentDto.getFromAmount());
            debit(bankTarget, paymentDto.getToAmount().subtract(commission));
            credit(to, paymentDto.getToAmount().subtract(commission));
        }
        return new UpdatedBalanceResponseDto(from.getStanje(), to.getStanje());
    }


    @Transactional
    @Override
    public void transfer(Account sender, Account recipient, BankPaymentDto paymentDto) {
        debit(sender, paymentDto.getAmount());
        credit(recipient, paymentDto.getAmount());
        //return new UpdatedBalanceResponseDto(null,null);
    }

    @Transactional
    @Override
    public void creditTransactional(Account account, BigDecimal amount)
    {
        credit(account,amount);
    }

    @Transactional
    @Override
    public void debitTransactional(Account account, BigDecimal amount) {
      debit(account,amount);
    }

    @Transactional
    @Override
    public void withdrawOneSided(Account account, BigDecimal amount) {
        // GHI #199: jednostrana debit operacija za trade-leg klijentskog BUY-a.
        // Reuse postojece debit() primitive sa istim saldo/limit proverama;
        // bankin racun se NE dira (po PM direktivi).
        debit(account, amount);
    }

    @Transactional
    @Override
    public void depositOneSided(Account account, BigDecimal amount) {
        // GHI #199: jednostrana credit operacija za trade-leg klijentskog SELL-a.
        credit(account, amount);
    }


}
