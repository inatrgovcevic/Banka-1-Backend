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
 * REST klijent ka {@code banking-core-service} koji se koristi iz saga klasa
 * (PR_11 C11.3).
 *
 * <p>Sa @CircuitBreaker (50% fail rate na 10 zahteva → open na 30s) i @Retry
 * (3 attempts sa eksponencijalnim backoff-om), iz {@link
 * com.banka1.security.config.Resilience4jConfig} (PR_06 C6.6).
 *
 * <p>Sve metode su synchronous (.block()) jer saga handler-i ionako traju
 * vise sekundi i koriste sopstveni async pattern preko RabbitMQ-a.
 */
@Slf4j
@Component
@RequiredArgsConstructor
public class BankingCoreClient {

    private static final String CB_NAME = "banking-core";

    @Value("${services.banking-core.url:http://banking-core-service:8084}")
    private String baseUrl;

    @Value("${services.banking-core.internal-token:}")
    private String internalToken;

    private WebClient webClient() {
        return WebClient.builder()
                .baseUrl(baseUrl)
                .defaultHeader(HttpHeaders.AUTHORIZATION, "Bearer " + internalToken)
                .defaultHeader(HttpHeaders.CONTENT_TYPE, MediaType.APPLICATION_JSON_VALUE)
                .build();
    }

    /** Rezervise sredstva na racunu kupca (Step 1 OTC_EXERCISE). */
    @CircuitBreaker(name = CB_NAME)
    @Retry(name = CB_NAME)
    public ReservationResult reserveFunds(Long buyerId, BigDecimal amount, String correlationId) {
        log.info("[banking-core] reserveFunds buyerId={} amount={} correlationId={}", buyerId, amount, correlationId);
        return webClient().post()
                .uri("/transactions/internal/reserve-funds")
                .header("X-Correlation-Id", correlationId)
                .bodyValue(Map.of("ownerId", buyerId, "amount", amount))
                .retrieve()
                .bodyToMono(ReservationResult.class)
                .timeout(Duration.ofSeconds(5))
                .block();
    }

    /** Oslobadja prethodno rezervisana sredstva (kompenzacija Step 1). */
    @CircuitBreaker(name = CB_NAME)
    @Retry(name = CB_NAME)
    public void releaseFunds(String reservationId, String correlationId) {
        log.info("[banking-core] releaseFunds reservation={} correlationId={}", reservationId, correlationId);
        webClient().delete()
                .uri("/transactions/internal/reservations/{id}", reservationId)
                .header("X-Correlation-Id", correlationId)
                .retrieve()
                .bodyToMono(Void.class)
                .timeout(Duration.ofSeconds(5))
                .block();
    }

    /** Stvarni transfer izmedju dva interna racuna (Step 3 OTC_EXERCISE, OTC_PREMIUM_TRANSFER, FUND_*). */
    @CircuitBreaker(name = CB_NAME)
    @Retry(name = CB_NAME)
    public TransferResult internalTransfer(String fromAccount, String toAccount, BigDecimal amount, String correlationId) {
        log.info("[banking-core] internalTransfer from={} to={} amount={} correlationId={}",
                fromAccount, toAccount, amount, correlationId);
        return webClient().post()
                .uri("/transactions/internal/transfer")
                .header("X-Correlation-Id", correlationId)
                .bodyValue(Map.of(
                        "fromAccountNumber", fromAccount,
                        "toAccountNumber", toAccount,
                        "amount", amount))
                .retrieve()
                .bodyToMono(TransferResult.class)
                .timeout(Duration.ofSeconds(10))
                .block();
    }

    /** Reverzira transfer — kompenzacija Step 3. */
    @CircuitBreaker(name = CB_NAME)
    @Retry(name = CB_NAME)
    public void reverseTransfer(String transferId, String correlationId) {
        log.info("[banking-core] reverseTransfer id={} correlationId={}", transferId, correlationId);
        webClient().post()
                .uri("/transactions/internal/transfers/{id}/reverse", transferId)
                .header("X-Correlation-Id", correlationId)
                .retrieve()
                .bodyToMono(Void.class)
                .timeout(Duration.ofSeconds(5))
                .block();
    }

    /** Vraca account number za ID korisnika — koristi se u OTC_EXERCISE Step 1. */
    @CircuitBreaker(name = CB_NAME)
    @Retry(name = CB_NAME)
    public String resolveDefaultAccountNumber(Long ownerId) {
        return webClient().get()
                .uri("/accounts/internal/default/{ownerId}", ownerId)
                .retrieve()
                .bodyToMono(Map.class)
                .map(m -> (String) m.get("accountNumber"))
                .timeout(Duration.ofSeconds(5))
                .block();
    }

    public record ReservationResult(String reservationId, String status) {}
    public record TransferResult(String transferId, String status) {}
}
