package com.banka1.tradingservice.otc.service;

import com.banka1.order.client.StockClient;
import com.banka1.order.dto.StockListingDto;
import com.banka1.order.entity.Portfolio;
import com.banka1.order.repository.PortfolioRepository;
import lombok.RequiredArgsConstructor;
import lombok.extern.slf4j.Slf4j;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;

import java.util.Optional;

/**
 * Manages Portfolio.reservedQuantity and Portfolio.publicQuantity in response to
 * OTC contract lifecycle events, keeping the stock state model in sync:
 *
 * <pre>
 * Accept  → reservedQuantity += amount, publicQuantity -= amount
 * Exercise→ quantity -= amount, reservedQuantity -= amount  (handled by StockReservationService)
 * Expire  → reservedQuantity -= amount, publicQuantity += amount
 * Cancel  → reservedQuantity -= amount, publicQuantity += amount
 * </pre>
 */
@Slf4j
@Service
@RequiredArgsConstructor
public class OtcPortfolioService {

    private final PortfolioRepository portfolioRepository;
    private final StockClient stockClient;

    /** Called when a contract is created (offer accepted). Locks stocks for OTC commitment. */
    @Transactional
    public void reserveForContract(Long sellerId, String ticker, int amount) {
        findPortfolio(sellerId, ticker).ifPresentOrElse(p -> {
            int reserved = p.getReservedQuantity() == null ? 0 : p.getReservedQuantity();
            int pub = p.getPublicQuantity() == null ? 0 : p.getPublicQuantity();
            p.setReservedQuantity(reserved + amount);
            p.setPublicQuantity(Math.max(0, pub - amount));
            portfolioRepository.save(p);
            log.info("OTC reserve: seller={} ticker={} amount={} → reserved={} public={}",
                    sellerId, ticker, amount, p.getReservedQuantity(), p.getPublicQuantity());
        }, () -> log.warn("OTC reserve: no portfolio found for seller={} ticker={}", sellerId, ticker));
    }

    /** Called when a contract is CANCELED or EXPIRED. Releases previously reserved stocks. */
    @Transactional
    public void releaseForContract(Long sellerId, String ticker, int amount) {
        findPortfolio(sellerId, ticker).ifPresentOrElse(p -> {
            int reserved = p.getReservedQuantity() == null ? 0 : p.getReservedQuantity();
            int pub = p.getPublicQuantity() == null ? 0 : p.getPublicQuantity();
            int qty = p.getQuantity() == null ? 0 : p.getQuantity();
            p.setReservedQuantity(Math.max(0, reserved - amount));
            // Restore publicQuantity up to totalOwned (seller may have reduced it manually)
            p.setPublicQuantity(Math.min(qty, pub + amount));
            portfolioRepository.save(p);
            log.info("OTC release: seller={} ticker={} amount={} → reserved={} public={}",
                    sellerId, ticker, amount, p.getReservedQuantity(), p.getPublicQuantity());
        }, () -> log.warn("OTC release: no portfolio found for seller={} ticker={}", sellerId, ticker));
    }

    /** Returns OTC capacity for the seller+ticker: min(publicQuantity, quantity - reservedQuantity). */
    public long getOtcCapacity(Long sellerId, String ticker) {
        return findPortfolio(sellerId, ticker)
                .map(p -> {
                    int qty = p.getQuantity() == null ? 0 : p.getQuantity();
                    int res = p.getReservedQuantity() == null ? 0 : p.getReservedQuantity();
                    long available = Math.max(0, qty - res);
                    if (Boolean.TRUE.equals(p.getIsPublic()) && p.getPublicQuantity() != null) {
                        // publicQuantity is a user-controlled display cap; the hard cap is
                        // quantity - reservedQuantity. Take the min of both.
                        return Math.min(p.getPublicQuantity().longValue(), available);
                    }
                    return available;
                })
                .orElse(0L);
    }

    private Optional<Portfolio> findPortfolio(Long userId, String ticker) {
        for (Portfolio p : portfolioRepository.findByUserId(userId)) {
            try {
                StockListingDto listing = stockClient.getListing(p.getListingId());
                if (listing != null && ticker.equalsIgnoreCase(listing.getTicker())) {
                    return Optional.of(p);
                }
            } catch (Exception ignored) {}
        }
        return Optional.empty();
    }
}