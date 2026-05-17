package com.banka1.interbank.client;

import com.banka1.interbank.protocol.dto.CurrencyCode;
import java.math.BigDecimal;
import java.util.UUID;
import org.springframework.beans.factory.annotation.Qualifier;
import org.springframework.context.annotation.Profile;
import org.springframework.stereotype.Component;
import org.springframework.web.client.RestClient;

/**
 * PR_32 Phase 5: HTTP klijent ka {@code banking-core-service} za 2PC novcane
 * rezervacije i account resolve.
 *
 * <p>Sve metode su sinhrone — koordinator (Phase 7-9 TransactionCoordinator)
 * blokira do odgovora banking-core-a pre nego sto vrati {@code YES} vote
 * partner banci.
 *
 * <p>Endpoints (banking-core, Phase 11/15):
 * <ul>
 *     <li>{@code POST /internal/interbank/reserve-monas} — alokacija fonda na racunu</li>
 *     <li>{@code POST /internal/interbank/reservations/{id}/commit-monas} — finalizacija</li>
 *     <li>{@code DELETE /internal/interbank/reservations/{id}} — odbacivanje</li>
 *     <li>{@code GET  /internal/interbank/account-resolve?num=...} — vraca {ownerType,
 *         ownerId, currency, availableBalance} za inbound 2PC NO_SUCH_ACCOUNT /
 *         INSUFFICIENT_ASSET pre-validation (Phase 15 — premešteno iz trading-service-a
 *         jer accounts žive u banking-core).</li>
 * </ul>
 */
@Component
@Profile("!test")
public class BankingCoreInternalClient {

    private final RestClient client;

    public BankingCoreInternalClient(@Qualifier("bankingCoreRestClient") RestClient client) {
        this.client = client;
    }

    /**
     * Zahtev za rezervaciju novca na lokalnom racunu.
     *
     * @param accountNum            18-digit broj racuna (lokalni)
     * @param currency              valuta sredstava
     * @param amount                iznos za rezervaciju (mora biti pozitivan, BigDecimal precision)
     * @param transactionIdRouting  routing number inicijatora transakcije (za audit)
     * @param transactionIdLocal    lokalni ID transakcije inicijatora (za idempotency)
     */
    public record ReserveMonasReq(String accountNum,
                                  CurrencyCode currency,
                                  BigDecimal amount,
                                  int transactionIdRouting,
                                  String transactionIdLocal) {}

    public record ReserveMonasRes(UUID reservationId) {}

    public ReserveMonasRes reserveMonas(ReserveMonasReq req) {
        return client.post()
                .uri("/internal/interbank/reserve-monas")
                .body(req)
                .retrieve()
                .body(ReserveMonasRes.class);
    }

    public void commitMonas(UUID reservationId) {
        client.post()
                .uri("/internal/interbank/reservations/{id}/commit-monas", reservationId)
                .retrieve()
                .toBodilessEntity();
    }

    public void releaseMonas(UUID reservationId) {
        client.delete()
                .uri("/internal/interbank/reservations/{id}", reservationId)
                .retrieve()
                .toBodilessEntity();
    }

    // ===== Account resolve (Phase 15 — premešteno iz TradingInternalClient) ===

    /**
     * Rezultat {@code GET /internal/interbank/account-resolve?num=...}.
     *
     * <p>Banking-core vraca {@code currency} kao String (oznaka tipa "USD"/"EUR"/"RSD")
     * — ovde ga drzimo kao {@link CurrencyCode} jer koordinator radi sa enum-om.
     * RestClient konvertuje string -> enum kroz Jackson default mapping.
     */
    public record AccountResolveRes(String ownerType,
                                    Long ownerId,
                                    CurrencyCode currency,
                                    BigDecimal availableBalance) {}

    public AccountResolveRes resolveAccount(String accountNum) {
        return client.get()
                .uri(uriBuilder -> uriBuilder
                        .path("/internal/interbank/account-resolve")
                        .queryParam("num", accountNum)
                        .build())
                .retrieve()
                .body(AccountResolveRes.class);
    }
}
