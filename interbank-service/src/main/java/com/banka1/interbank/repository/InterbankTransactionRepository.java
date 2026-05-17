package com.banka1.interbank.repository;

import com.banka1.interbank.model.InterbankTransactionEntity;
import java.util.Optional;
import org.springframework.data.jpa.repository.JpaRepository;

/**
 * PR_32 Phase 3: Spring Data repo za {@link InterbankTransactionEntity}.
 *
 * <p>Lookup po kanonskom 2PC kljucu {@code (transactionIdRouting,
 * transactionIdLocal)} je jedini glavni use-case — COMMIT_TX / ROLLBACK_TX
 * koordinator-i koriste ovo da locate prepared transaction po payload-u koji
 * je partnerska banka poslala.
 */
public interface InterbankTransactionRepository extends JpaRepository<InterbankTransactionEntity, Long> {

    Optional<InterbankTransactionEntity> findByTransactionIdRoutingAndTransactionIdLocal(
        int transactionIdRouting,
        String transactionIdLocal
    );
}
