package com.banka1.interbank.mock;

import com.banka1.interbank.otc.dto.PublicStockEntryDto;
import com.banka1.interbank.otc.dto.PublicStockSellerDto;
import com.banka1.interbank.protocol.dto.ForeignBankId;
import com.banka1.interbank.protocol.dto.InterbankMessagePayload;
import com.banka1.interbank.protocol.dto.NoVoteReason;
import com.banka1.interbank.protocol.dto.StockDescription;
import com.banka1.interbank.protocol.dto.TransactionVote;
import com.fasterxml.jackson.databind.ObjectMapper;
import java.util.List;
import java.util.Map;
import java.util.UUID;
import lombok.RequiredArgsConstructor;
import lombok.extern.slf4j.Slf4j;
import org.springframework.context.annotation.Profile;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.DeleteMapping;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.PathVariable;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.PutMapping;
import org.springframework.web.bind.annotation.RequestBody;
import org.springframework.web.bind.annotation.RequestHeader;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RequestParam;
import org.springframework.web.bind.annotation.RestController;

/**
 * PR_32 Phase 9: Mock Banka 2 controller za end-to-end testiranje bez stvarne
 * Banka 2 instance.
 *
 * <p>Aktivan SAMO kad je {@code mock-partner} Spring profile aktiviran. U
 * default {@code dev} profilu, interbank-service postavlja
 * {@code BANKA2_BASE_URL=http://localhost:8091/_mock/banka2/} → outbound pozivi
 * "ka Banka 2" stizu nazad u nas vlastiti proces i obrade se ovde.
 *
 * <p>Mocked endpoint-i pokrivaju 3 podsistema:
 * <ul>
 *   <li><b>2PC inter-bank protokol</b> ({@code POST /interbank}): NEW_TX (default
 *       YES vote, ali sa {@code ?vote=NO&reason=...} query parametrima moze da
 *       simulira NO odgovor sa konkretnim razlogom), COMMIT_TX/ROLLBACK_TX
 *       (uvek 204).</li>
 *   <li><b>Public stock</b> ({@code GET /public-stock}): canned lista od 2
 *       ticker-a (TSLA, MSFT) sa po jednim prodavcem.</li>
 *   <li><b>Negotiations</b> ({@code /negotiations/**}): create/counter/get/delete/
 *       accept — canned reply-evi, X-Api-Key validacija za mutirajuce verbe.</li>
 *   <li><b>User lookup</b> ({@code GET /user/{rn}/{id}}): canned display name.</li>
 * </ul>
 *
 * <p><strong>NIJE production code.</strong> Auth validacija je deliberatno
 * povrsna ({@code apiKey == null || apiKey.isBlank()} → 401) — pravi inter-bank
 * filter (per Phase 4) pokriva produkciju kroz {@code InterbankAuthFilter} na
 * stvarnoj Banka 2 strani. Ovaj mock postoji samo da omoguci local dev loop
 * ({@code curl + docker compose up}) bez deploy-a partner servisa.
 */
@RestController
@RequestMapping("/_mock/banka2")
@Profile("mock-partner")
@RequiredArgsConstructor
@Slf4j
public class MockBanka2Controller {

    private static final int BANKA2_ROUTING_NUMBER = 222;
    private static final int BANKA1_ROUTING_NUMBER = 111;

    @SuppressWarnings("unused")
    private final ObjectMapper mapper;

    /** Simulira POST /interbank na Banka 2 strani. */
    @PostMapping("/interbank")
    public ResponseEntity<?> mockInterbank(
            @RequestHeader(value = "X-Api-Key", required = false) String apiKey,
            @RequestBody InterbankMessagePayload msg,
            @RequestParam(defaultValue = "YES") String vote,
            @RequestParam(required = false) String reason) {

        if (apiKey == null || apiKey.isBlank()) {
            return ResponseEntity.status(401).build();
        }

        log.info("MockBanka2 received {} key={}", msg.messageType(),
                msg.idempotenceKey().locallyGeneratedKey());

        return switch (msg.messageType()) {
            case NEW_TX -> {
                if ("NO".equalsIgnoreCase(vote)) {
                    NoVoteReason.Reason r = NoVoteReason.Reason.valueOf(
                            reason != null ? reason : "INSUFFICIENT_ASSET");
                    yield ResponseEntity.ok(TransactionVote.no(List.of(NoVoteReason.of(r))));
                }
                yield ResponseEntity.ok(TransactionVote.yes());
            }
            case COMMIT_TX, ROLLBACK_TX -> ResponseEntity.noContent().build();
        };
    }

    /** Simulira GET /public-stock na Banka 2 strani. */
    @GetMapping("/public-stock")
    public ResponseEntity<List<PublicStockEntryDto>> mockPublicStock() {
        return ResponseEntity.ok(List.of(
                new PublicStockEntryDto(
                        new StockDescription("TSLA"),
                        List.of(new PublicStockSellerDto(
                                new ForeignBankId(BANKA2_ROUTING_NUMBER, "C-99"), 25))),
                new PublicStockEntryDto(
                        new StockDescription("MSFT"),
                        List.of(new PublicStockSellerDto(
                                new ForeignBankId(BANKA2_ROUTING_NUMBER, "C-100"), 50)))
        ));
    }

    /** Simulira POST /negotiations — Banka 2 vraca novokreirano neg-id. */
    @PostMapping("/negotiations")
    public ResponseEntity<ForeignBankId> mockCreateNegotiation(
            @RequestHeader(value = "X-Api-Key", required = false) String apiKey,
            @RequestBody Map<String, Object> offer) {
        if (apiKey == null || apiKey.isBlank()) {
            return ResponseEntity.status(401).build();
        }
        return ResponseEntity.ok(new ForeignBankId(
                BANKA2_ROUTING_NUMBER,
                "mock-neg-" + UUID.randomUUID().toString().substring(0, 8)));
    }

    @PutMapping("/negotiations/{rn}/{id}")
    public ResponseEntity<Void> mockCounterOffer(
            @PathVariable int rn,
            @PathVariable String id,
            @RequestHeader(value = "X-Api-Key", required = false) String apiKey,
            @RequestBody Map<String, Object> offer) {
        if (apiKey == null || apiKey.isBlank()) {
            return ResponseEntity.status(401).build();
        }
        return ResponseEntity.noContent().build();
    }

    @GetMapping("/negotiations/{rn}/{id}")
    public ResponseEntity<Map<String, Object>> mockGetNegotiation(
            @PathVariable int rn, @PathVariable String id) {
        // Vrati canned data za negotiation lookup.
        return ResponseEntity.ok(Map.of(
                "stock", Map.of("ticker", "AAPL"),
                "settlementDate", "2026-06-15T00:00:00+02:00",
                "pricePerUnit", Map.of("currency", "USD", "amount", 195.00),
                "premium", Map.of("currency", "USD", "amount", 500.00),
                "buyerId", Map.of("routingNumber", BANKA2_ROUTING_NUMBER, "id", "C-99"),
                "sellerId", Map.of("routingNumber", BANKA1_ROUTING_NUMBER, "id", "C-5"),
                "amount", 10,
                "lastModifiedBy", Map.of("routingNumber", BANKA1_ROUTING_NUMBER, "id", "C-5"),
                "isOngoing", true
        ));
    }

    @DeleteMapping("/negotiations/{rn}/{id}")
    public ResponseEntity<Void> mockDeleteNegotiation(
            @PathVariable int rn, @PathVariable String id) {
        return ResponseEntity.noContent().build();
    }

    @GetMapping("/negotiations/{rn}/{id}/accept")
    public ResponseEntity<Void> mockAcceptNegotiation(
            @PathVariable int rn, @PathVariable String id) {
        return ResponseEntity.noContent().build();
    }

    @GetMapping("/user/{rn}/{id}")
    public ResponseEntity<Map<String, String>> mockUser(
            @PathVariable int rn, @PathVariable String id) {
        return ResponseEntity.ok(Map.of(
                "bankDisplayName", "Banka 2",
                "displayName", "Mock " + id
        ));
    }
}
