package com.banka1.banking_service.card_service.rest_client;

import com.banka1.banking_service.card_service.security.JWTService;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import org.springframework.web.client.RestClient;

/**
 * HTTP client configuration for internal service-to-service calls.
 */
@Configuration
public class RestClientConfig {

    @Bean
    public RestClient clientServiceClient(
            @Value("${services.client.url}") String baseUrl,
            JWTService jwtService
    ) {
        return RestClient.builder()
                .baseUrl(baseUrl)
                .requestInterceptor(new JwtAuthInterceptor(jwtService))
                .build();
    }

    @Bean
    public RestClient accountServiceClient(
            @Value("${services.account.url}") String baseUrl,
            JWTService jwtService
    ) {
        return RestClient.builder()
                .baseUrl(baseUrl)
                .requestInterceptor(new JwtAuthInterceptor(jwtService))
                .build();
    }

    @Bean("cardVerificationClient")
    public RestClient cardVerificationClient(
            @Value("${services.verification.url}") String baseUrl,
            JWTService jwtService
    ) {
        return RestClient.builder()
                .baseUrl(baseUrl)
                .requestInterceptor(new JwtAuthInterceptor(jwtService))
                .build();
    }
}
