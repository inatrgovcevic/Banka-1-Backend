package com.banka1.banking_service.credit_service.rest_client;



import com.banka1.banking_service.credit_service.domain.enums.CurrencyCode;
import com.banka1.banking_service.credit_service.dto.response.ConversionResponseDto;
import org.springframework.beans.factory.annotation.Qualifier;
import org.springframework.stereotype.Service;
import org.springframework.web.client.RestClient;

import java.math.BigDecimal;

/**
 * REST Client for communicating with the Exchange Service.
 * Provides methods for currency conversion calculations.
 */
@Service
public class ExchangeService {
    private final RestClient restClient;

    /**
     * Constructs ExchangeService with a qualified RestClient bean.
     *
     * @param restClient the RestClient bean configured for exchange service communication
     */
    public ExchangeService(@Qualifier("creditExchangeClient") RestClient restClient) {
        this.restClient = restClient;
    }

    /**
     * Calculates currency conversion for a given amount.
     *
     * @param fromCurrency the source currency code
     * @param toCurrency the target currency code
     * @param amount the amount to convert
     * @return ConversionResponseDto containing conversion details and results
     */
    public ConversionResponseDto calculate(CurrencyCode fromCurrency,
                                           CurrencyCode toCurrency,
                                           BigDecimal amount) {
        return restClient.get()
                .uri(uriBuilder -> uriBuilder
                        .path("/calculate")
                        .queryParam("fromCurrency", fromCurrency.name())
                        .queryParam("toCurrency", toCurrency.name())
                        .queryParam("amount", amount)
                        .build())
                .retrieve()
                .body(ConversionResponseDto.class);
    }



}
