package com.banka1.interbank.client;

import org.springframework.beans.factory.annotation.Qualifier;
import org.springframework.context.annotation.Profile;
import org.springframework.stereotype.Component;
import org.springframework.web.client.RestClient;

/**
 * PR_32 Phase 5: HTTP klijent ka {@code user-service} za resolve display
 * imena (ime + prezime) lokalnih korisnika/zaposlenih.
 *
 * <p>Koristi se kad inbound public-stock / negotiations request donosi
 * foreign bank ID-evi pa interbank-service mora da ih obogati ljudski
 * citljivim imenima pre nego sto ih vrati (audit + UI).
 */
@Component
@Profile("!test")
public class UserInternalClient {

    private final RestClient client;

    public UserInternalClient(@Qualifier("userRestClient") RestClient client) {
        this.client = client;
    }

    /**
     * Response za resolve usera. {@code fullName} je vec sklepan na strani
     * user-service-a da bismo izbegli string concat ovde.
     */
    public record UserDisplayRes(String firstName, String lastName, String fullName) {}

    /**
     * Resolve display imena.
     *
     * @param type {@code CLIENT} ili {@code EMPLOYEE} (case-insensitive — server
     *             ocekuje uppercase, mi normalizujemo)
     * @param id   lokalni numericki ID korisnika u {@code user-service}-u
     */
    public UserDisplayRes resolveUser(String type, Long id) {
        return client.get()
                .uri("/internal/interbank/user/{t}/{i}", type.toUpperCase(), id)
                .retrieve()
                .body(UserDisplayRes.class);
    }
}
