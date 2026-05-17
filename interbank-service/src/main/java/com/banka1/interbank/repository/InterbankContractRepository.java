package com.banka1.interbank.repository;

import com.banka1.interbank.model.InterbankContractEntity;
import com.banka1.interbank.model.enums.NegotiationContractStatus;
import java.time.OffsetDateTime;
import java.util.List;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.Query;
import org.springframework.data.repository.query.Param;

/**
 * PR_32 Phase 3: Spring Data repo za {@link InterbankContractEntity}.
 *
 * <p>Glavni use-case-ovi:
 * <ul>
 *   <li>{@link #findByStatusAndSettlementDateBefore} — expiry scheduler hvata
 *       ACTIVE ugovore ciji je settlement_date prosao i flipuje ih u
 *       EXPIRED.</li>
 *   <li>{@link #sumActiveBySellerAndTicker} — KRIT #3 invariant: koliko
 *       akcija seller "drzi" u ACTIVE ugovorima za dati ticker. Pre
 *       prihvatanja nove ponude treba da vazi
 *       {@code sumActive + amount <= portfolio.quantity}.</li>
 * </ul>
 */
public interface InterbankContractRepository extends JpaRepository<InterbankContractEntity, String> {

    List<InterbankContractEntity> findByStatusAndSettlementDateBefore(
        NegotiationContractStatus status,
        OffsetDateTime cutoff
    );

    @Query("SELECT COALESCE(SUM(c.amount), 0) FROM InterbankContractEntity c "
        + "WHERE c.sellerRoutingNumber = :rn AND c.sellerId = :id "
        + "AND c.stockTicker = :ticker "
        + "AND c.status = com.banka1.interbank.model.enums.NegotiationContractStatus.ACTIVE")
    long sumActiveBySellerAndTicker(
        @Param("rn") int sellerRoutingNumber,
        @Param("id") String sellerId,
        @Param("ticker") String stockTicker
    );
}
