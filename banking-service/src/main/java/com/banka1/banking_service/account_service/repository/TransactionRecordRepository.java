package com.banka1.banking_service.account_service.repository;

import com.banka1.banking_service.account_service.domain.TransactionRecord;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.stereotype.Repository;

/**
 * Spring Data JPA repository za upravljanje transakcijskim zapisima.
 * <p>
 * Omogućava čuvanje i pronalaženje zapisa o sve izvršenim transakcijama.
 * Svaki zapis sadrži detaljne informacije o transakciji za audit trail i
 * rekoncijaciju sa drugim servisima.
 * <p>
 * Ova baza se koristi kao tamošnja istorija svih finansijskih operacija
 * na računima u ovoj servisnoj instanci. Iz tog razloga, zapisi se obično
 * samo kreiraju i čitaju, a ne ažuriraju ili brišu.
 */
@Repository
public interface TransactionRecordRepository extends JpaRepository<TransactionRecord, Long> {
}
