package com.banka1.banking_service.account_service.service.implementation;

import com.banka1.banking_service.account_service.domain.Currency;
import com.banka1.banking_service.account_service.domain.enums.CurrencyCode;
import com.banka1.banking_service.account_service.domain.enums.Status;
import com.banka1.banking_service.account_service.repository.CurrencyRepository;
import com.banka1.banking_service.account_service.service.CurrencyService;
import lombok.RequiredArgsConstructor;
import org.springframework.data.domain.Page;
import org.springframework.data.domain.PageRequest;
import org.springframework.stereotype.Service;

import java.util.List;

/**
 * Implementacija servisa za pristup podacima o valutama.
 * <p>
 * Učitava samo aktivne valute iz baze podataka. Valute se učitavaju
 * putem Liquibase seed migracija pri pokretanju aplikacije.
 */
@Service
@RequiredArgsConstructor
public class CurrencyServiceImplementation implements CurrencyService {
    /** Repozitorijum za pristup valutama iz baze. */
    private final CurrencyRepository currencyRepository;

    /**
     * Vraca sve aktivne valute kao listu.
     *
     * @return lista svih aktivnih {@link Currency} objekata
     */
    @Override
    public List<Currency> findAll() {
        return currencyRepository.findByStatus(Status.ACTIVE);
    }

    /**
     * Vraca paginiranu listu aktivnih valuta.
     *
     * @param page broj stranice, 0-indeksiran
     * @param size broj valuta po stranici
     * @return stranica sa aktivnim valutama
     */
    @Override
    public Page<Currency> findAllPage(int page, int size) {
        return currencyRepository.findByStatus(Status.ACTIVE,PageRequest.of(page,size));
    }

    /**
     * Pronalazi valutu po ISO kodu.
     *
     * @param code ISO kod valute (npr. RSD, EUR, USD)
     * @return {@link Currency} objekat sa traženim kodom
     * @throws RuntimeException ako valuta sa datim kodom ne postoji
     */
    @Override
    public Currency findByCode(CurrencyCode code) {
        return currencyRepository.findByOznaka(code).orElseThrow((() -> new RuntimeException("Currency nije pronadjen")));
    }


}
