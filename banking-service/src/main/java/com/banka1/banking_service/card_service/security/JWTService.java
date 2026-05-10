package com.banka1.banking_service.card_service.security;

/**
 * Generates internal service JWT tokens for authenticated inter-service calls.
 */
public interface JWTService {

    /**
     * Generates a signed service JWT.
     *
     * @return signed bearer token
     */
    String generateJwtToken();
}
