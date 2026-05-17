package com.banka1.tradingservice.interbank.service;

import com.banka1.order.client.StockClient;
import com.banka1.order.dto.StockListingDto;
import com.banka1.order.entity.Portfolio;
import com.banka1.order.repository.PortfolioRepository;
import com.banka1.tradingservice.interbank.model.InterbankStockReservation;
import com.banka1.tradingservice.interbank.repository.InterbankStockReservationRepository;
import lombok.RequiredArgsConstructor;
import lombok.extern.slf4j.Slf4j;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;

import java.time.Instant;
import java.util.UUID;

/**
 * Trading-service servis koji ekspozira interbank 2PC primitive za akcije
 * (PR_32 Phase 12).
 *
 * <p>Pozivaju ga {@code /internal/interbank/*} endpoint-i koje interbank-service
 * gadja kroz {@code TradingInternalClient}. Servis radi nad legacy
 * {@link Portfolio} entity-jem (trading-service ucitava {@code order-service}
 * kao library — PR_19 C19.X), bez REST hop-a.
 *
 * <p>Pattern za balanse:
 * <ul>
 *   <li>{@code reserveStock} — povecava {@code portfolio.reservedQuantity},
 *       ne dira {@code portfolio.quantity}. Owner i dalje vidi pozicije ali
 *       ne moze prodavati rezervisane jedinice.</li>
 *   <li>{@code commitStock} — skida i {@code quantity} i {@code reservedQuantity}.
 *       Akcije su trajno presle drugom korisniku.</li>
 *   <li>{@code releaseStock} — samo {@code reservedQuantity} se vraca (jer
 *       quantity nije nikad smanjen).</li>
 * </ul>
 */
@Slf4j
@Service
@RequiredArgsConstructor
public class InterbankStockReservationService {

    public static final String STATUS_HELD = "HELD";
    public static final String STATUS_COMMITTED = "COMMITTED";
    public static final String STATUS_RELEASED = "RELEASED";

    private final InterbankStockReservationRepository reservationRepository;
    private final PortfolioRepository portfolioRepository;
    private final StockClient stockClient;

    /**
     * Rezervise akcije na prodavcevom portfoliu (HELD).
     *
     * @param ownerUserId lokalni user id prodavca
     * @param ticker simbol akcije; resolve-uje se na listing kroz stock-service
     * @param quantity broj jedinica
     * @param transactionIdRouting interbank routing number iz Tim 2 protokola
     * @param transactionIdLocal lokalni dio kljuca
     * @return UUID nove rezervacije
     * @throws IllegalArgumentException ako pozicija ne postoji ili nema dovoljno
     *         raspolozivih akcija ({@code quantity - reservedQuantity}).
     */
    @Transactional
    public UUID reserveStock(Long ownerUserId,
                             String ticker,
                             int quantity,
                             int transactionIdRouting,
                             String transactionIdLocal) {
        if (quantity <= 0) {
            throw new IllegalArgumentException("Quantity must be positive: " + quantity);
        }
        if (ticker == null || ticker.isBlank()) {
            throw new IllegalArgumentException("Ticker must not be blank");
        }

        Portfolio portfolio = findPortfolioByOwnerAndTickerForUpdate(ownerUserId, ticker)
                .orElseThrow(() -> new IllegalArgumentException(
                        "No portfolio position for user=" + ownerUserId + " ticker=" + ticker));

        int total = portfolio.getQuantity() == null ? 0 : portfolio.getQuantity();
        int reserved = portfolio.getReservedQuantity() == null ? 0 : portfolio.getReservedQuantity();
        int available = total - reserved;
        if (available < quantity) {
            throw new IllegalArgumentException(
                    "Insufficient stock for reservation: have=" + available
                            + " need=" + quantity + " (ticker=" + ticker + ")");
        }

        portfolio.setReservedQuantity(reserved + quantity);
        portfolioRepository.save(portfolio);

        UUID reservationId = UUID.randomUUID();
        InterbankStockReservation reservation = InterbankStockReservation.builder()
                .reservationId(reservationId)
                .transactionIdRouting(transactionIdRouting)
                .transactionIdLocal(transactionIdLocal)
                .portfolioId(portfolio.getId())
                .ticker(ticker)
                .quantity(quantity)
                .status(STATUS_HELD)
                .build();
        reservationRepository.save(reservation);

        log.info("Interbank reserveStock: owner={} ticker={} qty={} txRouting={} txLocal={} resId={}",
                ownerUserId, ticker, quantity, transactionIdRouting, transactionIdLocal, reservationId);
        return reservationId;
    }

    /**
     * 2PC commit: skida quantity i reservedQuantity. Idempotentno.
     *
     * @throws IllegalArgumentException ako rezervacija ne postoji.
     * @throws IllegalStateException ako je rezervacija RELEASED.
     */
    @Transactional
    public void commitStock(UUID reservationId) {
        InterbankStockReservation reservation = reservationRepository.findByReservationId(reservationId)
                .orElseThrow(() -> new IllegalArgumentException(
                        "Stock reservation not found: " + reservationId));

        if (STATUS_COMMITTED.equals(reservation.getStatus())) {
            log.info("Interbank commitStock: reservation {} already COMMITTED — no-op", reservationId);
            return;
        }
        if (!STATUS_HELD.equals(reservation.getStatus())) {
            throw new IllegalStateException(
                    "Cannot commit reservation " + reservationId + " in state " + reservation.getStatus());
        }

        Portfolio portfolio = portfolioRepository.findById(reservation.getPortfolioId())
                .orElseThrow(() -> new IllegalArgumentException(
                        "Portfolio vanished: id=" + reservation.getPortfolioId()));
        int total = portfolio.getQuantity() == null ? 0 : portfolio.getQuantity();
        int reserved = portfolio.getReservedQuantity() == null ? 0 : portfolio.getReservedQuantity();
        int qty = reservation.getQuantity();

        portfolio.setQuantity(Math.max(0, total - qty));
        portfolio.setReservedQuantity(Math.max(0, reserved - qty));
        portfolioRepository.save(portfolio);

        reservation.setStatus(STATUS_COMMITTED);
        reservation.setFinalizedAt(Instant.now());
        reservationRepository.save(reservation);

        log.info("Interbank commitStock: reservation={} portfolio={} qty={}",
                reservationId, portfolio.getId(), qty);
    }

    /**
     * 2PC abort: vraca reservedQuantity. Idempotentno.
     *
     * @throws IllegalArgumentException ako rezervacija ne postoji.
     * @throws IllegalStateException ako je rezervacija COMMITTED.
     */
    @Transactional
    public void releaseStock(UUID reservationId) {
        InterbankStockReservation reservation = reservationRepository.findByReservationId(reservationId)
                .orElseThrow(() -> new IllegalArgumentException(
                        "Stock reservation not found: " + reservationId));

        if (STATUS_RELEASED.equals(reservation.getStatus())) {
            log.info("Interbank releaseStock: reservation {} already RELEASED — no-op", reservationId);
            return;
        }
        if (STATUS_COMMITTED.equals(reservation.getStatus())) {
            throw new IllegalStateException(
                    "Cannot release reservation " + reservationId + " — already COMMITTED");
        }

        Portfolio portfolio = portfolioRepository.findById(reservation.getPortfolioId())
                .orElseThrow(() -> new IllegalArgumentException(
                        "Portfolio vanished: id=" + reservation.getPortfolioId()));
        int reserved = portfolio.getReservedQuantity() == null ? 0 : portfolio.getReservedQuantity();
        int qty = reservation.getQuantity();

        portfolio.setReservedQuantity(Math.max(0, reserved - qty));
        portfolioRepository.save(portfolio);

        reservation.setStatus(STATUS_RELEASED);
        reservation.setFinalizedAt(Instant.now());
        reservationRepository.save(reservation);

        log.info("Interbank releaseStock: reservation={} portfolio={} qty={}",
                reservationId, portfolio.getId(), qty);
    }

    /**
     * Pretrazi sve pozicije date osobe i nadji onu cija listing ima dati ticker.
     * Resolve listingId -> ticker kroz stock-service. Vraca pessimistic-write
     * lock zakljucanu poziciju.
     */
    private java.util.Optional<Portfolio> findPortfolioByOwnerAndTickerForUpdate(Long userId, String ticker) {
        for (Portfolio p : portfolioRepository.findByUserId(userId)) {
            try {
                StockListingDto listing = stockClient.getListing(p.getListingId());
                if (listing != null && ticker.equalsIgnoreCase(listing.getTicker())) {
                    return portfolioRepository.findByUserIdAndListingIdForUpdate(userId, p.getListingId());
                }
            } catch (Exception ignored) {
                // Listing nije dostupan — preskoci.
            }
        }
        return java.util.Optional.empty();
    }
}
