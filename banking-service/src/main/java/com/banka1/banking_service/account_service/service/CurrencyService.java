package com.banka1.banking_service.account_service.service;

import com.banka1.banking_service.account_service.domain.Currency;
import com.banka1.banking_service.account_service.domain.enums.CurrencyCode;
import org.springframework.data.domain.Page;

import java.util.List;

/**
 * Servis za pristup podacima o valutama koje banka podrzava.
 */
public interface CurrencyService {

    /**
     * Vraca sve valute bez paginacije.
     *
     * @return lista svih valuta
     */
    List<Currency> findAll();

    /**
     * Vraca paginiranu listu valuta.
     *
     * @param page broj stranice (0-indeksiran)
     * @param size velicina stranice
     * @return stranica valuta
     */
    Page<Currency> findAllPage(int page, int size);

    /**
     * Pronalazi valutu po kodu (npr. RSD, EUR, USD).
     *
     * @param code kod valute
     * @return entitet valute
     * @throws IllegalArgumentException ako valuta sa datim kodom ne postoji
     */
    Currency findByCode(CurrencyCode code);
}
