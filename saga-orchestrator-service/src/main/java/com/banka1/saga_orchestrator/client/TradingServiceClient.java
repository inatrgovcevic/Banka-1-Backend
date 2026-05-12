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
 * REST klijent ka {@code trading-service} (PR_15 C15.3).
 *
 * <p>Pre PR_15 saga {@code FundRedeemWithLiquidationSaga} je zvala MarketServiceClient
 * za likvidaciju fonda. Domenski netacno: FundHolding entitet i logika "prodaj N
 * hartija fonda" zivi u trading-service-u, ne u market-service-u (koji drzi samo
 * stock price feed).
 */
@Slf4j
@Component
@RequiredArgsConstructor
public class TradingServiceClient {

    private static final String CB_NAME = "trading-service";

    @Value("${services.trading.url:http://trading-service:8086}")
    private String baseUrl;

    @Value("${services.trading.internal-token:}")
    private String internalToken;

    private WebClient webClient() {
        return WebClient.builder()
                .baseUrl(baseUrl)
                .defaultHeader(HttpHeaders.AUTHORIZATION, "Bearer " + internalToken)
                .defaultHeader(HttpHeaders.CONTENT_TYPE, MediaType.APPLICATION_JSON_VALUE)
                .build();
    }

    /**
     * FUND_LIQUIDATION_FOR_REDEMPTION step 1: trading-service likvidira hartije fonda
     * dok ne pokrije zadati iznos. Posto FundHolding entitet zivi u trading-service-u
     * (PR_14 C14.7), endpoint je tamo, ne u market-service-u.
     */
    @CircuitBreaker(name = CB_NAME)
    @Retry(name = CB_NAME)
    public LiquidationResult liquidateForFund(Long fundId, BigDecimal targetAmount, String correlationId) {
        log.info("[trading-service] liquidateForFund fund={} amount={} correlationId={}",
                fundId, targetAmount, correlationId);
        return webClient().post()
                .uri("/funds/internal/{fundId}/liquidate", fundId)
                .header("X-Correlation-Id", correlationId)
                .bodyValue(Map.of("targetAmount", targetAmount))
                .retrieve()
                .bodyToMono(LiquidationResult.class)
                .timeout(Duration.ofSeconds(30))
                .block();
    }

    public record LiquidationResult(String liquidationId, BigDecimal liquidatedAmount, int holdingsSold) {}
}
