package com.banka1.tradingservice.security;

/**
 * Service-to-service JWT generator za trading-service (PR_15 C15.4).
 *
 * <p>Koristi se kada async kontekst (npr. RabbitMQ listener) treba da pozove
 * druge servise — u tim slucajevima nema korisnikovog JWT-a u SecurityContext-u
 * pa AccountServiceClient.forwardToken() pristup ne radi.
 */
public interface JWTService {
    String generateJwtToken();
}
