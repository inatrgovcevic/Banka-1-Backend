package com.banka1.tradingservice.otc.repository;

import com.banka1.tradingservice.otc.domain.OtcOffer;
import com.banka1.tradingservice.otc.domain.OtcOfferStatus;
import jakarta.persistence.LockModeType;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.Lock;
import org.springframework.data.jpa.repository.Query;
import org.springframework.stereotype.Repository;

import java.time.LocalDate;
import java.util.List;

@Repository
public interface OtcOfferRepository extends JpaRepository<OtcOffer, Long> {

    /** Pesimisticki lock za accept — sprecava race condition dva istovremena prihvatanja. */
    @Lock(LockModeType.PESSIMISTIC_WRITE)
    @Query("SELECT o FROM OtcOffer o WHERE o.id = :id")
    java.util.Optional<OtcOffer> findByIdForUpdate(@org.springframework.data.repository.query.Param("id") Long id);

    /** Aktivne ponude za korisnika (kupca ili prodavca) — Stranica: Aktivne ponude. */
    List<OtcOffer> findByBuyerIdAndStatusInOrSellerIdAndStatusIn(
            Long buyerId, List<OtcOfferStatus> buyerStatuses,
            Long sellerId, List<OtcOfferStatus> sellerStatuses
    );

    /** Bulk lookup za expired sweeper cron. */
    List<OtcOffer> findByStatusInAndSettlementDateBefore(List<OtcOfferStatus> statuses, LocalDate before);

    /**
     * Suma kolicina svih aktivnih pregovora (PENDING_BUYER/PENDING_SELLER) gde je
     * dati user prodavac za dati ticker, iskljucujuci konkretnu ponudu koja se upravo prihvata.
     * Koristi se u invariant proveri pri prihvatanju — pregovori takodje zauzimaju kapacitet.
     */
    @org.springframework.data.jpa.repository.Query(
            "SELECT COALESCE(SUM(o.amount), 0) FROM OtcOffer o "
            + "WHERE o.sellerId = :sellerId AND o.stockTicker = :ticker "
            + "AND o.id != :excludeOfferId "
            + "AND o.status IN (com.banka1.tradingservice.otc.domain.OtcOfferStatus.PENDING_BUYER, "
            + "                 com.banka1.tradingservice.otc.domain.OtcOfferStatus.PENDING_SELLER)")
    long sumPendingBySellerAndTickerExcluding(
            @org.springframework.data.repository.query.Param("sellerId") Long sellerId,
            @org.springframework.data.repository.query.Param("ticker") String ticker,
            @org.springframework.data.repository.query.Param("excludeOfferId") Long excludeOfferId);
}
