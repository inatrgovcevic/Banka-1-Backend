package com.banka1.banking_service.transfer_service.security;

/**
 * Servisni interfejs zadužen za operacije generisanja sistemskih JWT tokena.
 * Koristi se za autentifikaciju "servis-servis" komunikacije.
 */
public interface JWTService {
    /**
     * Generiše sistemski JWT token sa "SERVICE" ulogom.
     * @return serijalizovani JWT string spreman za Authorization zaglavlje
     */
    String generateJwtToken();
}