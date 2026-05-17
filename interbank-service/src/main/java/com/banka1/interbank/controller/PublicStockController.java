package com.banka1.interbank.controller;

import com.banka1.interbank.client.TradingInternalClient;
import com.banka1.interbank.otc.dto.PublicStockEntryDto;
import com.banka1.interbank.repository.InterbankContractRepository;
import java.util.List;
import lombok.RequiredArgsConstructor;
import lombok.extern.slf4j.Slf4j;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RestController;

/**
 * PR_32 Phase 10 Task 10.5: GET /public-stock per Tim 2 §3.1.
 *
 * <p>Vraca listu svih ticker-a koje nasi klijenti i zaposleni nude za OTC
 * prodaju, zajedno sa kolicinom. Quantity = portfolio.quantity -
 * sum(active ACTIVE option contract reservations) — KRIT #3 invariant
 * (CLAUDE.md sekcija 4).
 *
 * <p>{@link TradingInternalClient#getPublicStocks()} vec radi quantity
 * agregaciju u trading-service-u; ovde dodatno odbijamo {@code amount}
 * iz {@link InterbankContractRepository#sumActiveBySellerAndTicker} za
 * svakog seller-a, zato sto trading endpoint cuva samo intra-bank
 * rezervacije, a kontrakti su u interbank-service-u.
 */
@RestController
@RequiredArgsConstructor
@Slf4j
public class PublicStockController {

    private final TradingInternalClient trading;
    private final InterbankContractRepository contractRepo;

    /**
     * §3.1 GET /public-stock — vraca listu hartija + svojih prodavaca.
     * Auth: X-Api-Key (InterbankAuthFilter, vidi Phase 4).
     */
    @GetMapping("/public-stock")
    public List<PublicStockEntryDto> publicStock() {
        List<PublicStockEntryDto> raw = trading.getPublicStocks();
        if (raw == null || raw.isEmpty()) {
            return List.of();
        }
        // Subtract active option contract reservations per ticker per seller
        return raw.stream()
                .map(entry -> {
                    String ticker = entry.stock().ticker();
                    var filtered = entry.sellers().stream()
                            .map(s -> {
                                long reserved = contractRepo.sumActiveBySellerAndTicker(
                                        s.seller().routingNumber(),
                                        s.seller().id(),
                                        ticker);
                                int available = (int) Math.max(0L, s.amount() - reserved);
                                return new com.banka1.interbank.otc.dto.PublicStockSellerDto(
                                        s.seller(), available);
                            })
                            .filter(s -> s.amount() > 0)
                            .toList();
                    return new PublicStockEntryDto(entry.stock(), filtered);
                })
                .filter(e -> !e.sellers().isEmpty())
                .toList();
    }
}
