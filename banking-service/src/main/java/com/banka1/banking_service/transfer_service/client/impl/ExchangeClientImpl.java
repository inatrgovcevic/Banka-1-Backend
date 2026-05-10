package com.banka1.banking_service.transfer_service.client.impl;

import com.banka1.banking_service.transfer_service.client.ExchangeClient;
import com.banka1.banking_service.transfer_service.dto.client.ExchangeResponseDto;
import com.banka1.banking_service.transfer_service.exception.BusinessException;
import com.banka1.banking_service.transfer_service.exception.ErrorCode;
import lombok.RequiredArgsConstructor;
import lombok.extern.slf4j.Slf4j;
import org.springframework.context.annotation.Profile;
import org.springframework.stereotype.Component;
import org.springframework.web.client.HttpClientErrorException;
import org.springframework.web.client.HttpServerErrorException;
import org.springframework.web.client.RestClient;

import java.math.BigDecimal;


/**
 * Implementacija klijenta za menjačnicu koja koristi query parametre za kalkulaciju kursa.
 */
@Component
@Profile("!local")
@RequiredArgsConstructor
@Slf4j
public class ExchangeClientImpl implements ExchangeClient {

    private final RestClient exchangeRestClient;

    @Override
    public ExchangeResponseDto calculateExchange(String fromCurrency, String toCurrency, BigDecimal amount) {
        try {
            return exchangeRestClient.get()
                    .uri(uriBuilder -> uriBuilder
                            .path("/calculate")
                            .queryParam("fromCurrency", fromCurrency)
                            .queryParam("toCurrency", toCurrency)
                            .queryParam("amount", amount)
                            .build())
                    .retrieve()
                    .body(ExchangeResponseDto.class);
        } catch (HttpClientErrorException e) {
            // Ako menjačnica vrati 4xx, verovatno je neispravna valuta
            log.error("[{}] Exchange service 4xx error: {}", e.getClass().getSimpleName(), e.getResponseBodyAsString());
            throw new BusinessException(ErrorCode.ACCOUNT_NOT_FOUND, "Greška u menjačnici: " + e.getResponseBodyAsString());
        } catch (HttpServerErrorException e) {
            log.error("[{}] Exchange service 5xx error ({}): {}", e.getClass().getSimpleName(), e.getStatusCode(), e.getResponseBodyAsString());
            throw new BusinessException(ErrorCode.TRANSFER_NOT_FOUND, "Servis menjačnice vratio grešku: " + e.getStatusCode());
        } catch (Exception e) {
            log.error("[{}] Exchange service unavailable: {}", e.getClass().getSimpleName(), e.getMessage(), e);
            throw new BusinessException(ErrorCode.TRANSFER_NOT_FOUND, "Servis menjačnice trenutno nije dostupan.");
        }
    }
}