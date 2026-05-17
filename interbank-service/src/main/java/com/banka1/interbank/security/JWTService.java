package com.banka1.interbank.security;

/**
 * PR_32 Phase 4: Service-to-service JWT generator za interbank-service.
 *
 * <p>Koristi se kada interbank-service pravi OUTBOUND HTTP poziv ka drugim
 * internim servisima (banking-core, trading, user) i nema korisnikovog JWT-a
 * u SecurityContext-u (npr. async kontekst RabbitMQ listenera ili sched
 * job-a).
 *
 * <p>Token ima subject = "interbank-service" i role "SERVICE", potpisan sa
 * shared HS256 secret-om ({@code jwt.secret}). security-lib na drugoj strani
 * priznaje role SERVICE kroz role hijerarhiju (vidi
 * {@code SecurityConfig.apiChain}).
 */
public interface JWTService {
    String generateJwtToken();
}
