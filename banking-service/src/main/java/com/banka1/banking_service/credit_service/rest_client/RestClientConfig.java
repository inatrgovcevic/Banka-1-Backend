package com.banka1.banking_service.credit_service.rest_client;

import com.banka1.banking_service.credit_service.security.JWTService;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import org.springframework.web.client.RestClient;

/**
 * Configuration class for REST client beans.
 * Creates and configures RestClient instances for communicating with external services
 * (User, Verification, Exchange, and Account services).
 * All clients include JWT authentication interceptor.
 */
@Configuration
public class RestClientConfig {
    /**
     * Creates a RestClient.Builder bean.
     *
     * @return RestClient.Builder for constructing configured REST clients
     */
    /**
     * Creates a RestClient configured for User/Client Service communication.
     *
//     * @param builder the RestClient builder
     * @param baseUrl the base URL for the user service
     * @param jwtService the JWT service for token generation
     * @return configured RestClient for user service
     */
    @Bean("creditUserClient")
    public RestClient creditUserClient(
            @Value("${services.user.url}") String baseUrl,
            JWTService jwtService
    ) {
        return RestClient.builder()
                .baseUrl(baseUrl)
                .requestInterceptor(new JwtAuthInterceptor(jwtService))
                .build();
    }


    /**
     * Creates a RestClient configured for Verification Service communication.
     *
//     * @param builder the RestClient builder
     * @param baseUrl the base URL for the verification service
     * @param jwtService the JWT service for token generation
     * @return configured RestClient for verification service
     */
    @Bean("creditVerificationClient")
    public RestClient creditVerificationClient(
            @Value("${services.verification.url}") String baseUrl,
            JWTService jwtService
    ) {
        return RestClient.builder()
                .baseUrl(baseUrl)
                .requestInterceptor(new JwtAuthInterceptor(jwtService))
                .build();
    }


    /**
     * Creates a RestClient configured for Exchange Service communication.
     *
//     * @param builder the RestClient builder
     * @param baseUrl the base URL for the exchange service
     * @param jwtService the JWT service for token generation
     * @return configured RestClient for exchange service
     */
    @Bean("creditExchangeClient")
    public RestClient creditExchangeClient(
            @Value("${services.exchange.url}") String baseUrl,
            JWTService jwtService
    ) {
        return RestClient.builder()
                .baseUrl(baseUrl)
                .requestInterceptor(new JwtAuthInterceptor(jwtService))
                .build();
    }

    /**
     * Creates a RestClient configured for Account Service communication.
     *
//     * @param builder the RestClient builder
     * @param baseUrl the base URL for the account service
     * @param jwtService the JWT service for token generation
     * @return configured RestClient for account service
     */
    @Bean("creditAccountClient")
    public RestClient creditAccountClient(
            @Value("${services.account.url}") String baseUrl,
            JWTService jwtService
    ) {
        return RestClient.builder()
                .baseUrl(baseUrl)
                .requestInterceptor(new JwtAuthInterceptor(jwtService))
                .build();
    }


}
