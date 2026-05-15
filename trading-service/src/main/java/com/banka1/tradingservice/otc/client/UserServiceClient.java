package com.banka1.tradingservice.otc.client;

import lombok.extern.slf4j.Slf4j;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.core.ParameterizedTypeReference;
import org.springframework.http.HttpHeaders;
import org.springframework.http.MediaType;
import org.springframework.stereotype.Component;
import org.springframework.web.reactive.function.client.WebClient;

import java.time.Duration;
import java.util.Collections;
import java.util.List;

/**
 * HTTP klijent ka user-service-u za OTC interne operacije.
 */
@Slf4j
@Component
public class UserServiceClient {

    private final WebClient webClient;

    public UserServiceClient(@Value("${services.user-service.url:http://user-service:8081}") String baseUrl) {
        this.webClient = WebClient.builder()
                .baseUrl(baseUrl)
                .defaultHeader(HttpHeaders.CONTENT_TYPE, MediaType.APPLICATION_JSON_VALUE)
                .build();
    }

    /**
     * Vraca klijentske ID-eve svih aktuara (zaposleni sa role=AGENT).
     * Koristi se za supervisor view filtriranje OTC public stocks.
     */
    public List<Long> getActuaryClientIds() {
        try {
            List<Long> ids = webClient.get()
                    .uri("/internal/otc/actuary-client-ids")
                    .retrieve()
                    .bodyToMono(new ParameterizedTypeReference<List<Long>>() {})
                    .timeout(Duration.ofSeconds(5))
                    .block();
            return ids != null ? ids : Collections.emptyList();
        } catch (Exception e) {
            log.warn("Failed to fetch actuary client IDs from user-service: {}", e.getMessage());
            return Collections.emptyList();
        }
    }
}