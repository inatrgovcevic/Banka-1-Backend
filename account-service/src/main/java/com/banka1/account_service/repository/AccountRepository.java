package com.banka1.account_service.repository;

import com.banka1.account_service.domain.Account;
import com.banka1.account_service.domain.CheckingAccount;
import com.banka1.account_service.domain.Currency;
import com.banka1.account_service.domain.SystemAccountIds;
import com.banka1.account_service.domain.enums.CardStatus;
import com.banka1.account_service.domain.enums.CurrencyCode;
import com.banka1.account_service.domain.enums.Status;
import org.springframework.data.domain.Page;
import org.springframework.data.domain.Pageable;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.Modifying;
import org.springframework.data.jpa.repository.Query;
import org.springframework.data.repository.query.Param;
import org.springframework.stereotype.Repository;

import java.util.List;
import java.util.Optional;

/**
 * Spring Data JPA repository za upravljanje bankovskim računima.
 * <p>
 * Omogućava jednostavno pronalaženje, pretragu i ažuriranje računa sa standardnim
 * i prilagođenim upitima. Podržava pronalaženje po raznim kriterijumima
 * (broj računa, vlasnik, valuta, status) i operacije batch ažuriranja
 * (reset dnevne i mesečne potrošnje).
 * <p>
 * Napomena: Neki upiti trebalo bi da se filtriraju po aktivnim računima.
 */
@Repository
public interface AccountRepository extends JpaRepository<Account, Long> {
    /**
     * Proverava da li račun sa datim brojem već postoji.
     *
     * @param brojRacuna broj računa za pretragu
     * @return {@code true} ako račun postoji, {@code false} inače
     */
    boolean existsByBrojRacuna(String brojRacuna);

    /**
     * Proverava da li vlasnik već ima račun sa datim nazivom.
     *
     * @param vlasnik ID vlasnika računa
     * @param nazivRacuna naziv računa
     * @return {@code true} ako postoji račun sa tim nazivom za vlasnika, {@code false} inače
     */
    boolean existsByVlasnikAndNazivRacuna(Long vlasnik, String nazivRacuna);

    /**
     * Pronalazi sve račune vlasnika sa datim statusom, paginirano.
     *
     * @param id ID vlasnika
     * @param status status računa
     * @param pageable parametri paginacije
     * @return stranica sa računima koji zadovoljavaju uslov
     */
    Page<Account> findByVlasnikAndStatus(Long id, Status status, Pageable pageable);

    /**
     * Pronalazi račun po broju računa.
     *
     * @param brojRacuna jedinstveni 19-cifreni broj računa
     * @return {@code Optional} sa računom ako postoji
     */
    Optional<Account> findByBrojRacuna(String brojRacuna);

    /**
     * Pronalazi račun po ID-u i valuti.
     *
     * @param id ID računa
     * @param currency valuta
     * @return {@code Optional} sa računom ako postoji
     */
    Optional<Account> findByIdAndCurrency(Long id, Currency currency);

    /**
     * Pronalazi prvi račun vlasnika sa datom valutom.
     *
     * @param vlasnik ID vlasnika
     * @param currency valuta
     * @return {@code Optional} sa računom ako postoji
     */
    Optional<Account> findByVlasnikAndCurrency(Long vlasnik, Currency currency);

    /**
     * Pronalazi sve aktivne tekuće račune koji imaju uslugu održavanja računa (sa gebinom > 0).
     * <p>
     * Koristi se za batch obračun mesečnih gebrina.
     *
     * @return lista aktivnih tekućih računa sa gebinom
     */
    @Query("""
        SELECT a
        FROM CheckingAccount a
        WHERE a.status = com.banka1.account_service.domain.enums.Status.ACTIVE
          AND a.odrzavanjeRacuna IS NOT NULL
          AND a.odrzavanjeRacuna > 0
    """)
    List<CheckingAccount> findAllActiveCheckingAccountsWithMaintenanceFee();

    /**
     * Resetuje dnevnu potrošnju na 0 za sve račune.
     * <p>
     * Koristi se kao batch operacija koja se obično izvršava jednom dnevno preko scheduler-a.
     *
     * @return broj ažuriranih redova
     */
    @Modifying
    @Query("""
        UPDATE Account a
        SET a.dnevnaPotrosnja = 0
    """)
    int resetDailySpending();

    /**
     * Resetuje mesečnu potrošnju na 0 za sve račune.
     * <p>
     * Koristi se kao batch operacija koja se obično izvršava prvi dan meseca preko scheduler-a.
     *
     * @return broj ažuriranih redova
     */
    @Modifying
    @Query("""
        UPDATE Account a
        SET a.mesecnaPotrosnja = 0
    """)
    int resetMonthlySpending();

    /**
     * Pretražuje račune po broju računa i imenu vlasnika sa case-insensitive podudaranjem.
     * <p>
     * Svi parametri su opcioni i koriste se kao wildcard pretrage sa LIKE operatorom.
     * Rezultati su sortirani po prezimenu i imenu vlasnika.
     *
     * @param brojRacuna parcijalni broj računa za pretragu (opciono)
     * @param ime parcijalno ime vlasnika za pretragu (opciono)
     * @param prezime parcijalno prezime vlasnika za pretragu (opciono)
     * @param pageable parametri paginacije
     * @return stranica sa računima koji zadovoljavaju uslov
     */
    @Query("""
    SELECT a FROM Account a
    WHERE LOWER(a.brojRacuna) LIKE LOWER(CONCAT('%', COALESCE(:brojRacuna, ''), '%'))
    AND LOWER(a.imeVlasnikaRacuna) LIKE LOWER(CONCAT('%', COALESCE(:ime, ''), '%'))
    AND LOWER(a.prezimeVlasnikaRacuna) LIKE LOWER(CONCAT('%', COALESCE(:prezime, ''), '%'))
    ORDER BY a.prezimeVlasnikaRacuna ASC, a.imeVlasnikaRacuna ASC, a.id ASC
""")
    /* PR_31 hotfix: dodato a.id ASC kao tie-breaker. Bez njega svi bankarski
       racuni (prezime=ime=Banka) imaju isti ORDER BY kljuc → PostgreSQL vraca
       ne-deterministicki redosled → LIMIT/OFFSET pagination duplira i preskace
       redove (npr. AUD dva puta, USD nikad). a.id je primary key, garantuje
       stabilan total order. */
    Page<Account> searchAccounts(
            @Param("brojRacuna") String brojRacuna,
            @Param("ime") String ime,
            @Param("prezime") String prezime,
            Pageable pageable
    );

    /**
     * Pronalazi sve racune sa zadatim ID-em vlasnika.
     *
     * <p>PR_29: Bazna metoda kojoj delegiraju {@code findAllBankAccounts()},
     * {@code findBankAccountByCurrencyCode()} i {@code findStateAccountByCurrencyCode()}
     * — umesto da svaka tri JPQL upita ima hardkodirano {@code -1} / {@code -2}.
     */
    @Query("SELECT a FROM Account a WHERE a.vlasnik = :vlasnik")
    List<Account> findByVlasnik(@Param("vlasnik") Long vlasnik);

    /**
     * Pronalazi sve racune sa zadatim vlasnikom i valutom.
     */
    @Query("SELECT a FROM Account a WHERE a.vlasnik = :vlasnik AND a.currency.oznaka = :currencyCode")
    Optional<Account> findByVlasnikAndCurrencyCode(@Param("vlasnik") Long vlasnik,
                                                   @Param("currencyCode") CurrencyCode currencyCode);

    /**
     * Pronalazi sve bankovne (sistemske) račune.
     * <p>
     * Bankovni računi se koriste kao soborna sredstva za razne operacije
     * (npr. prikupljanje gebrina, provizija). PR_29: konstanta {@link SystemAccountIds#BANK}
     * umesto magic broja {@code -1}.
     */
    default List<Account> findAllBankAccounts() {
        return findByVlasnik(SystemAccountIds.BANK);
    }

    /**
     * Pronalazi bankovni račun za datim kodom valute.
     *
     * @param currencyCode kod valute (npr. RSD, EUR, USD)
     * @return {@code Optional} sa bankovnim računom u datoj valuti
     */
    default Optional<Account> findBankAccountByCurrencyCode(CurrencyCode currencyCode) {
        return findByVlasnikAndCurrencyCode(SystemAccountIds.BANK, currencyCode);
    }

    /**
     * Pronalazi drzavni (State) racun za zadatom valutom.
     * <p>
     * Drzava je modelovana kao firma (Celina 3 — "Naša država = Firma") sa vlasnikom
     * {@link SystemAccountIds#STATE}, razlicitim od banke ({@link SystemAccountIds#BANK})
     * i regularnih klijenata (pozitivni ID-evi). Koristi se pri naplati poreza na
     * kapitalnu dobit i pri namirenju opcionih ugovora (exercise), gde prenos treba
     * da ide na drzavni racun, ne na racun banke.
     *
     * @param currencyCode kod valute (u praksi samo RSD)
     * @return {@code Optional} sa drzavnim racunom u datoj valuti
     */
    default Optional<Account> findStateAccountByCurrencyCode(CurrencyCode currencyCode) {
        return findByVlasnikAndCurrencyCode(SystemAccountIds.STATE, currencyCode);
    }
}
