package com.banka1.banking_service.transfer_service.repository;

import com.banka1.banking_service.transfer_service.domain.Transfer;
import org.springframework.data.domain.Page;
import org.springframework.data.domain.Pageable;
import org.springframework.data.jpa.repository.JpaRepository;

import java.util.Optional;

/**
 * Repozitorijum za upravljanje perzistencijom {@link Transfer} entiteta.
 * Sadrži metode za pretragu istorije transfera po klijentu, računu i poslovnom broju naloga.
 */
public interface TransferRepository extends JpaRepository<Transfer, Long> {
    /**
     * Pronalazi transfer na osnovu njegovog jedinstvenog poslovnog broja naloga.
     * @param orderNumber jedinstveni identifikator transfera (npr. TRF-XXXX)
     * @return Optional koji sadrži transfer ako je pronađen u bazi
     */
    Optional<Transfer> findByOrderNumber(String orderNumber);

    /**
     * Vraća paginiranu listu svih transfera iniciranih od strane specifičnog klijenta.
     * @param clientId ID klijenta čija se istorija potražuje
     * @param pageable parametri za paginaciju i sortiranje
     * @return stranica sa rezultatima transfera za datog klijenta
     */
    Page<Transfer> findByClientId(Long clientId, Pageable pageable);
    /**
     * Proverava postojanje transfera sa specifičnim ID-em verifikacione sesije.
     * Koristi se kao mehanizam idempotencije kako bi se sprečilo duplo izvršavanje istog zahteva.
     * @param verificationSessionId jedinstveni ID sesije iz zahteva
     * @return true ako je transfer sa ovom sesijom već uspešno snimljen
     */
    boolean existsByVerificationSessionId(String verificationSessionId);

    /**
     * Pronalazi sve transfere u kojima je određeni bankovni račun učestvovao ili kao pošiljalac ili kao primalac.
     * @param fromAccountNumber broj računa pošiljaoca
     * @param toAccountNumber broj računa primaoca
     * @param pageable parametri paginacije
     * @return stranica sa istorijom transfera vezanih za traženi broj računa
     */
    Page<Transfer> findByFromAccountNumberOrToAccountNumber(String fromAccountNumber, String toAccountNumber, Pageable pageable);
}
