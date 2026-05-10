package com.banka1.banking_service.account_service.repository;

import com.banka1.banking_service.account_service.domain.Currency;
import com.banka1.banking_service.account_service.domain.enums.CurrencyCode;
import com.banka1.banking_service.account_service.domain.enums.Status;
import org.springframework.data.domain.Page;
import org.springframework.data.domain.Pageable;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.stereotype.Repository;

import java.util.List;
import java.util.Optional;

/**
 * Spring Data JPA repository za upravljanje valutama.
 * <p>
 * Omogućava pronalaženje i upravljanje valutama dostupnim u bankovnom sistemu.
 * Svaka valuta ima kod (RSD, EUR, USD, itd.), naziv i status (aktivna/neaktivna).
 * Koristi se pri kreiranju računa, konverziji sredstava i prikazivanju dostupnih opcija.
 */
@Repository
public interface CurrencyRepository extends JpaRepository<Currency, Long> {
    /**
     * Pronalazi valutu po kodu.
     *
     * @param oznaka kod valute (npr. RSD, EUR, USD, GBP)
     * @return {@code Optional} sa valutom ako postoji
     */
    Optional<Currency> findByOznaka(CurrencyCode oznaka);

    /**
     * Pronalazi sve valute sa datim statusom bez paginacije.
     * <p>
     * Koristi se kada je potrebna kompletna lista valuta (obično je mali broj).
     * Za veće podatke, koristi {@link #findByStatus(Status, Pageable)}.
     *
     * @param status status valute (ACTIVE, INACTIVE)
     * @return lista valuta sa datim statusom
     */
    List<Currency> findByStatus(Status status);

    /**
     * Pronalazi sve valute sa datim statusom, paginirano.
     * <p>
     * Koristi se ako se broj valuta prosledi ili se želi paginirana prezentacija.
     *
     * @param status status valute (ACTIVE, INACTIVE)
     * @param pageable parametri paginacije
     * @return stranica sa valutama sa datim statusom
     */
    Page<Currency> findByStatus(Status status, Pageable pageable);
}
