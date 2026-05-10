package com.banka1.banking_service.transfer_service.config;

import com.banka1.banking_service.transfer_service.security.JWTService;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.mockito.Mock;
import org.mockito.MockitoAnnotations;
import org.springframework.http.HttpHeaders;
import org.springframework.http.HttpRequest;
import org.springframework.http.client.ClientHttpRequestExecution;
import org.springframework.web.client.RestClient;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertNotNull;
import static org.mockito.Mockito.*;

class RestClientConfigTest {

    @Mock
    private JWTService jwtService;
    @Mock
    private RestClient.Builder restClientBuilder;
    @Mock
    private HttpRequest httpRequest;
    @Mock
    private ClientHttpRequestExecution execution;

    private RestClientConfig restClientConfig;

    @BeforeEach
    void setUp() {
        MockitoAnnotations.openMocks(this);
        restClientConfig = new RestClientConfig(jwtService);

        // Mocking RestClient.Builder as it's used to build clients
        when(restClientBuilder.baseUrl(anyString())).thenReturn(restClientBuilder);
        when(restClientBuilder.build()).thenReturn(mock(RestClient.class));
    }

    @Test
    void restClientBuilder_ShouldAddInterceptor() {
        // Mock restClientBuilder behavior
        when(restClientBuilder.requestInterceptor(any())).thenReturn(restClientBuilder);

        RestClient.Builder result = restClientConfig.restClientBuilder();

        assertNotNull(result);
    }

    @Test
    void accountRestClient_ShouldSetBaseUrl() {
        String baseUrl = "http://account-service";
        restClientConfig.accountRestClient(restClientBuilder, baseUrl);

        verify(restClientBuilder).baseUrl(baseUrl);
        verify(restClientBuilder).build();
    }

    /**
     * Testira JwtAuthInterceptor unutar RestClientConfig-a.
     * Koristimo refleksiju jer je klasa private.
     */
    @Test
    void jwtAuthInterceptor_ShouldAddAuthorizationHeader() throws Exception {
        String mockToken = "super-secret-token";
        when(jwtService.generateJwtToken()).thenReturn(mockToken);

        HttpHeaders headers = new HttpHeaders();
        when(httpRequest.getHeaders()).thenReturn(headers);

        // Instanciranje private unutrašnje klase preko refleksije (ili testiranjem same logike)
        // Ovde možemo direktno testirati interceptor ako ga izvučemo
        var interceptor = restClientConfig.new JwtAuthInterceptor();

        interceptor.intercept(httpRequest, new byte[0], execution);

        assertEquals("Bearer " + mockToken, headers.getFirst(HttpHeaders.AUTHORIZATION));
        verify(execution).execute(httpRequest, new byte[0]);
    }
}