package com.banka1.banking_service.transfer_service.client.impl;


import com.banka1.banking_service.transfer_service.dto.client.ExchangeResponseDto;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import static org.junit.jupiter.api.Assertions.*;
import org.mockito.Mock;
import org.mockito.MockitoAnnotations;
import org.springframework.web.client.RestClient;

import java.math.BigDecimal;
import java.util.function.Function;

import static org.junit.jupiter.api.Assertions.assertNotNull;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

class ExchangeClientImplTest {

    private ExchangeClientImpl exchangeClient;

    @Mock
    private RestClient restClient;

    @Mock
    private RestClient.RequestHeadersUriSpec requestHeadersUriSpec;

    @Mock
    private RestClient.ResponseSpec responseSpec;

    @BeforeEach
    void setUp() {
        MockitoAnnotations.openMocks(this);
        exchangeClient = new ExchangeClientImpl(restClient);
    }

    @Test
    void calculateExchange_Success() {
        ExchangeResponseDto expected = new ExchangeResponseDto(
                "RSD", "EUR", new BigDecimal("1170"), new BigDecimal("10"), new BigDecimal("117"), BigDecimal.ZERO
        );

        when(restClient.get()).thenReturn(requestHeadersUriSpec);
        // Ovde mokujemo uri metodu koja prihvata Function (uriBuilder)
        when(requestHeadersUriSpec.uri(any(Function.class))).thenReturn(requestHeadersUriSpec);
        when(requestHeadersUriSpec.retrieve()).thenReturn(responseSpec);
        when(responseSpec.body(ExchangeResponseDto.class)).thenReturn(expected);

        ExchangeResponseDto result = exchangeClient.calculateExchange("RSD", "EUR", new BigDecimal("1170"));

        assertNotNull(result);
        assertEquals(new BigDecimal("117"), result.rate());
        verify(restClient).get();
    }
}