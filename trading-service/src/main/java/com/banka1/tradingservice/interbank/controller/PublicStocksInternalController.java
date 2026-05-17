package com.banka1.tradingservice.interbank.controller;

import com.banka1.order.client.StockClient;
import com.banka1.order.dto.StockListingDto;
import com.banka1.order.entity.Portfolio;
import com.banka1.order.repository.PortfolioRepository;
import lombok.RequiredArgsConstructor;
import lombok.extern.slf4j.Slf4j;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.http.ResponseEntity;
import org.springframework.security.access.prepost.PreAuthorize;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;

import java.util.ArrayList;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;

/**
 * GET /internal/interbank/public-stocks — vraca listu javno-objavljenih
 * STOCK pozicija u nasoj banci, grupisanih po ticker-u (PR_32 Phase 12/15, Tim 2 §11).
 *
 * <p>Format odgovora poklapa interbank-service {@code PublicStockEntryDto}
 * i {@code PublicStockSellerDto}: lokalni DTO-i su isto-imenovani parovi
 * (ticker -> lista prodavaca sa {routingNumber, id} foreign-bank tagom).
 *
 * <p>Autorizacija: {@code hasRole('SERVICE')} (interbank-service service JWT).
 */
@Slf4j
@RestController
@RequestMapping("/internal/interbank")
@PreAuthorize("hasRole('SERVICE')")
@RequiredArgsConstructor
public class PublicStocksInternalController {

    private final PortfolioRepository portfolioRepository;
    private final StockClient stockClient;

    @Value("${interbank.my-routing-number:111}")
    private int myRoutingNumber;

    public record StockDescription(String ticker) {}

    public record ForeignBankId(int routingNumber, String id) {}

    public record PublicStockSeller(ForeignBankId seller, int amount) {}

    public record PublicStockEntry(StockDescription stock, List<PublicStockSeller> sellers) {}

    @GetMapping("/public-stocks")
    public ResponseEntity<List<PublicStockEntry>> getPublicStocks() {
        Map<String, List<PublicStockSeller>> byTicker = new LinkedHashMap<>();

        for (Portfolio p : portfolioRepository.findAllPublicStocks()) {
            String ticker = resolveTicker(p.getListingId());
            if (ticker == null) {
                continue;
            }
            int amount = p.getPublicQuantity() == null ? 0 : p.getPublicQuantity();
            if (amount <= 0) {
                continue;
            }
            // Per Tim 2 §3.2 spec, foreign-bank ID koristi prefiks "C-" za klijente
            // (i "E-" za zaposlene; vidi InternalUserDirectoryController u user-service).
            // Trading-service portfolio.userId je uvek klijent (ne employee), pa fiksiramo "C-" prefix.
            ForeignBankId fbId = new ForeignBankId(myRoutingNumber, "C-" + p.getUserId());
            byTicker
                    .computeIfAbsent(ticker, k -> new ArrayList<>())
                    .add(new PublicStockSeller(fbId, amount));
        }

        List<PublicStockEntry> entries = new ArrayList<>(byTicker.size());
        byTicker.forEach((ticker, sellers) ->
                entries.add(new PublicStockEntry(new StockDescription(ticker), sellers)));
        return ResponseEntity.ok(entries);
    }

    private String resolveTicker(Long listingId) {
        if (listingId == null) {
            return null;
        }
        try {
            StockListingDto listing = stockClient.getListing(listingId);
            return listing == null ? null : listing.getTicker();
        } catch (Exception e) {
            log.debug("Could not resolve ticker for listingId={}: {}", listingId, e.getMessage());
            return null;
        }
    }
}
