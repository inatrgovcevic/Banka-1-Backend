package com.banka1.tradingservice.otc.repository;

import com.banka1.tradingservice.otc.domain.OptionContract;
import com.banka1.tradingservice.otc.domain.OptionContractStatus;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.Query;
import org.springframework.data.repository.query.Param;
import org.springframework.stereotype.Repository;

import java.time.LocalDate;
import java.util.List;

@Repository
public interface OptionContractRepository extends JpaRepository<OptionContract, Long> {

    List<OptionContract> findByBuyerIdAndStatus(Long buyerId, OptionContractStatus status);
    List<OptionContract> findBySellerIdAndStatus(Long sellerId, OptionContractStatus status);

    /** Cron za auto-expire ugovora kojima je settlement date prosao. */
    List<OptionContract> findByStatusAndSettlementDateBefore(OptionContractStatus status, LocalDate before);

    /**
     * PR_32 Phase 12 KRIT #3: sumira amount-e svih jos uvek zivih ugovora
     * gde je dati user prodavac konkretnog ticker-a. Koristi se u
     * {@link com.banka1.tradingservice.otc.service.OtcService#accept(Long, Long)}
     * za reserved-stock invariant proveru pre prihvatanja nove ponude.
     *
     * <p>"Zivi" su ugovori u statusima {@code PENDING_PREMIUM} i {@code ACTIVE} —
     * oba blokiraju akcije prodavca (reservedQuantity).
     */
    @Query("SELECT COALESCE(SUM(c.amount), 0) FROM OptionContract c "
            + "WHERE c.sellerId = :sellerId AND c.stockTicker = :ticker "
            + "AND c.status IN (com.banka1.tradingservice.otc.domain.OptionContractStatus.ACTIVE, "
            + "                 com.banka1.tradingservice.otc.domain.OptionContractStatus.PENDING_PREMIUM)")
    long sumActiveBySellerAndTicker(@Param("sellerId") Long sellerId, @Param("ticker") String ticker);
}
