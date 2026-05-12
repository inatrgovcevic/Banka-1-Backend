package com.banka1.marketservice.stock.controller;

import com.banka1.marketservice.stock.dto.StockPriceSnapshotDto;
import com.banka1.marketservice.stock.service.StockPriceFeedService;
import lombok.RequiredArgsConstructor;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.*;

import java.util.List;

/**
 * Polling endpoint za current stock prices (PR_12 C12.1).
 *
 * <p>Frontend OtcOffersComponent / OtcContractsComponent koriste ovaj endpoint da
 * dohvate market price-ove koje koristi {@code deviation.ts} utility za color-coding
 * (zelena/zuta/crvena pragovi prema spec-u).
 *
 * <p>Polling interval na frontend strani: 30 s (Angular RxJS interval).
 * Backend keseira poslednju snapshot listu u memoriji 15 s da bi se izbegli
 * prekomerni AlphaVantage upiti.
 */
@RestController
@RequestMapping("/stocks/price-feed")
@RequiredArgsConstructor
public class StockPriceFeedController {

    private final StockPriceFeedService priceFeedService;

    /**
     * GET /stocks/price-feed/current?tickers=AAPL,MSFT,GOOGL
     * Vraca trenutne cene za zadate ticker-e (max 100 odjednom).
     */
    @GetMapping("/current")
    public ResponseEntity<List<StockPriceSnapshotDto>> currentPrices(@RequestParam List<String> tickers) {
        if (tickers == null || tickers.isEmpty() || tickers.size() > 100) {
            return ResponseEntity.badRequest().build();
        }
        return ResponseEntity.ok(priceFeedService.getCurrentPrices(tickers));
    }

    /**
     * GET /stocks/price-feed/single/{ticker}
     * Vraca cenu jednog ticker-a (sa cache-om).
     */
    @GetMapping("/single/{ticker}")
    public ResponseEntity<StockPriceSnapshotDto> currentPrice(@PathVariable String ticker) {
        StockPriceSnapshotDto snapshot = priceFeedService.getCurrentPrice(ticker);
        return snapshot != null ? ResponseEntity.ok(snapshot) : ResponseEntity.notFound().build();
    }
}
