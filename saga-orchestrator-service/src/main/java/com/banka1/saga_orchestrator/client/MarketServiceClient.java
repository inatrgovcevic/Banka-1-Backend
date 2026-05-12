package com.banka1.saga_orchestrator.client;

import io.github.resilience4j.circuitbreaker.annotation.CircuitBreaker;
import io.github.resilience4j.retry.annotation.Retry;
import lombok.RequiredArgsConstructor;
import lombok.extern.slf4j.Slf4j;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.http.HttpHeaders;
import org.springframework.http.MediaType;
import org.springframework.stereotype.Component;
import org.springframework.web.reactive.function.client.WebClient;

import java.math.BigDecimal;
import java.time.Duration;
import java.util.Map;

/**
 * REST klijent ka {@code market-service} (PR_11 C11.3).
 * Koristi se u OTC_EXERCISE step 2 i 4 (rezervacija/transfer hartija) i u
 * FUND_LIQUIDATION_FOR_REDEMPTION step 1 (likvidacija holdings-a fonda).
 */
@Slf4j
@Component
@RequiredArgsConstructor
public class MarketServiceClient {

    private static final String CB_NAME = "market-service";

    @Value("${services.market.url:http://market-service:8085}")
    private String baseUrl;

    @Value("${services.market.internal-token:}")
    private String internalToken;

    private WebClient webClient() {
        return WebClient.builder()
                .baseUrl(baseUrl)
                .defaultHeader(HttpHeaders.AUTHORIZATION, "Bearer " + internalToken)
                .defaultHeader(HttpHeaders.CONTENT_TYPE, MediaType.APPLICATION_JSON_VALUE)
                .build();
    }

    /** Step 2: Provera da prodavac stvarno ima trazene hartije + rezervacija. */
    @CircuitBreaker(name = CB_NAME)
    @Retry(name = CB_NAME)
    public StockReservationResult reserveStocks(Long sellerId, String stockTicker, int amount, String correlationId) {
        log.info("[market-service] reserveStocks seller={} ticker={} amount={} correlationId={}",
                sellerId, stockTicker, amount, correlationId);
        return webClient().post()
                .uri("/stocks/internal/reserve")
                .header("X-Correlation-Id", correlationId)
                .bodyValue(Map.of("ownerId", sellerId, "stockTicker", stockTicker, "amount", amount))
                .retrieve()
                .bodyToMono(StockReservationResult.class)
                .timeout(Duration.ofSeconds(5))
                .block();
    }

    /** Kompenzacija Step 2 — oslobadja rezervisane hartije. */
    @CircuitBreaker(name = CB_NAME)
    @Retry(name = CB_NAME)
    public void releaseStocks(String reservationId, String correlationId) {
        log.info("[market-service] releaseStocks reservation={} correlationId={}", reservationId, correlationId);
        webClient().delete()
                .uri("/stocks/internal/reservations/{id}", reservationId)
                .header("X-Correlation-Id", correlationId)
                .retrieve()
                .bodyToMono(Void.class)
                .timeout(Duration.ofSeconds(5))
                .block();
    }

    /** Step 4: Stvarni prenos vlasnistva nad hartijama sa prodavca na kupca. */
    @CircuitBreaker(name = CB_NAME)
    @Retry(name = CB_NAME)
    public OwnershipTransferResult transferOwnership(String reservationId, Long buyerId, String correlationId) {
        log.info("[market-service] transferOwnership reservation={} buyer={} correlationId={}",
                reservationId, buyerId, correlationId);
        return webClient().post()
                .uri("/stocks/internal/reservations/{id}/transfer", reservationId)
                .header("X-Correlation-Id", correlationId)
                .bodyValue(Map.of("buyerId", buyerId))
                .retrieve()
                .bodyToMono(OwnershipTransferResult.class)
                .timeout(Duration.ofSeconds(10))
                .block();
    }

    /** Kompenzacija Step 4 — vraca vlasnistvo prodavcu. */
    @CircuitBreaker(name = CB_NAME)
    @Retry(name = CB_NAME)
    public void reverseOwnership(String ownershipTransferId, String correlationId) {
        log.info("[market-service] reverseOwnership id={} correlationId={}", ownershipTransferId, correlationId);
        webClient().post()
                .uri("/stocks/internal/ownership-transfers/{id}/reverse", ownershipTransferId)
                .header("X-Correlation-Id", correlationId)
                .retrieve()
                .bodyToMono(Void.class)
                .timeout(Duration.ofSeconds(5))
                .block();
    }

    // PR_15 C15.3: liquidateForFund premesteno u TradingServiceClient
    // jer FundHolding entitet zivi u trading-service-u (PR_14 C14.7), a ne ovde.

    public record StockReservationResult(String reservationId, String status) {}
    public record OwnershipTransferResult(String ownershipTransferId, String status) {}
}
