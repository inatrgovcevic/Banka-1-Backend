package com.banka1.banking_service.transfer_service.client.impl;

import com.banka1.banking_service.transfer_service.dto.client.ClientInfoResponseDto;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import static org.junit.jupiter.api.Assertions.*;
import org.mockito.Mock;
import org.mockito.MockitoAnnotations;
import org.springframework.web.client.RestClient;

import static org.junit.jupiter.api.Assertions.assertNotNull;
import static org.mockito.ArgumentMatchers.anyLong;
import static org.mockito.ArgumentMatchers.anyString;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

class ClientClientImplTest {

    private ClientClientImpl clientClient;

    @Mock
    private RestClient restClient;

    @Mock
    private RestClient.RequestHeadersUriSpec requestHeadersUriSpec;

    @Mock
    private RestClient.ResponseSpec responseSpec;

    @BeforeEach
    void setUp() {
        MockitoAnnotations.openMocks(this);
        clientClient = new ClientClientImpl(restClient);
    }

    @Test
    void getClientDetails_Success() {
        Long clientId = 1L;
        ClientInfoResponseDto expected = new ClientInfoResponseDto(clientId, "Petar", "Petrovic", "petar@gmail.com");

        when(restClient.get()).thenReturn(requestHeadersUriSpec);
        when(requestHeadersUriSpec.uri(anyString(), anyLong())).thenReturn(requestHeadersUriSpec);
        when(requestHeadersUriSpec.retrieve()).thenReturn(responseSpec);
        when(responseSpec.body(ClientInfoResponseDto.class)).thenReturn(expected);

        ClientInfoResponseDto result = clientClient.getClientDetails(clientId);

        assertNotNull(result);
        assertEquals("Petar", result.getName());
        verify(restClient).get();
    }
}