package com.banka1.banking_service.card_service.security;

import org.springframework.beans.factory.annotation.Value;
import org.springframework.boot.autoconfigure.condition.ConditionalOnMissingBean;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import org.springframework.security.config.annotation.method.configuration.EnableMethodSecurity;
import org.springframework.security.oauth2.jwt.JwtDecoder;
import org.springframework.security.oauth2.jwt.NimbusJwtDecoder;

import javax.crypto.SecretKey;
import javax.crypto.spec.SecretKeySpec;

/**
 * Security configuration local to the card service.
 * This class exposes the beans that the module relies on directly.
 * It provides a JWT decoder for validating incoming access tokens.
 */
@Configuration
@EnableMethodSecurity
public class SecurityBeans {

    /**
     * Creates a JWT decoder backed by the shared HMAC secret.
     * Example:
     * a token signed with the configured {@code jwt.secret} can be validated successfully,
     * while a token signed with a different secret will fail validation.
     *
     * @param secret JWT signing secret loaded from configuration
     * @return configured JWT decoder used by Spring Security resource-server support
     */
    @Bean
    @ConditionalOnMissingBean(JwtDecoder.class)
    public JwtDecoder jwtDecoder(@Value("${jwt.secret}") String secret) {
        SecretKey key = new SecretKeySpec(secret.getBytes(), "HmacSHA256");
        return NimbusJwtDecoder.withSecretKey(key).build();
    }
}
