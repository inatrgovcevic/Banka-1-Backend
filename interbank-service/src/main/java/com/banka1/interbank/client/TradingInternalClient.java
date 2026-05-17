package com.banka1.interbank.client;

import com.banka1.interbank.otc.dto.PublicStockEntryDto;
import java.util.List;
import java.util.UUID;
import org.springframework.beans.factory.annotation.Qualifier;
import org.springframework.context.annotation.Profile;
import org.springframework.core.ParameterizedTypeReference;
import org.springframework.stereotype.Component;
import org.springframework.web.client.RestClient;

/**
 * PR_32 Phase 5: HTTP klijent ka {@code trading-service} za:
 * <ul>
 *     <li>2PC rezervaciju akcija u portfoliu prodavca,</li>
 *     <li>OTC opcioni ugovor lifecycle (reserve / exercise / release),</li>
 *     <li>public-stocks listu (Tim 2 §11).</li>
 * </ul>
 *
 * <p>Sve metode sinhrone — interbank koordinator blokira do response-a.
 *
 * <p>Account resolve je premešten u {@link BankingCoreInternalClient} (Phase 15)
 * jer accounts žive u banking-core servisu.
 */
@Component
@Profile("!test")
public class TradingInternalClient {

    private final RestClient client;

    public TradingInternalClient(@Qualifier("tradingRestClient") RestClient client) {
        this.client = client;
    }

    // ===== Stock 2PC =========================================================

    public record ReserveStockReq(Long sellerUserId,
                                  String ticker,
                                  int quantity,
                                  int transactionIdRouting,
                                  String transactionIdLocal) {}

    public record ReserveStockRes(UUID reservationId) {}

    public ReserveStockRes reserveStock(ReserveStockReq req) {
        return client.post()
                .uri("/internal/interbank/reserve-stock")
                .body(req)
                .retrieve()
                .body(ReserveStockRes.class);
    }

    public void commitStock(UUID reservationId) {
        client.post()
                .uri("/internal/interbank/reservations/{id}/commit-stock", reservationId)
                .retrieve()
                .toBodilessEntity();
    }

    public void releaseStock(UUID reservationId) {
        client.delete()
                .uri("/internal/interbank/reservations/{id}", reservationId)
                .retrieve()
                .toBodilessEntity();
    }

    // ===== OTC option lifecycle =============================================

    /**
     * Telo zahteva za rezervaciju akcija pod option ugovorom (per Tim 2 §15).
     * Negotiation ID je globalni (foreign-bank-id formata), seller je lokalni
     * foreign id u nasoj banci.
     */
    public record ReserveOptionReq(String sellerForeignId,
                                   String ticker,
                                   int quantity) {}

    public void reserveOption(String negotiationId, String sellerForeignId, String ticker, int quantity) {
        ReserveOptionReq body = new ReserveOptionReq(sellerForeignId, ticker, quantity);
        client.post()
                .uri("/internal/interbank/options/{id}/reserve", negotiationId)
                .body(body)
                .retrieve()
                .toBodilessEntity();
    }

    public void exerciseOption(String negotiationId) {
        client.post()
                .uri("/internal/interbank/options/{id}/exercise", negotiationId)
                .retrieve()
                .toBodilessEntity();
    }

    public void releaseOption(String negotiationId) {
        client.delete()
                .uri("/internal/interbank/options/{id}/release", negotiationId)
                .retrieve()
                .toBodilessEntity();
    }

    // ===== Public stocks listing =============================================

    public List<PublicStockEntryDto> getPublicStocks() {
        return client.get()
                .uri("/internal/interbank/public-stocks")
                .retrieve()
                .body(new ParameterizedTypeReference<List<PublicStockEntryDto>>() {});
    }
}
