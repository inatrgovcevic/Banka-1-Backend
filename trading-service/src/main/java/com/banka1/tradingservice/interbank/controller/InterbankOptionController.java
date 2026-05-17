package com.banka1.tradingservice.interbank.controller;

import com.banka1.tradingservice.interbank.service.InterbankStockReservationService;
import jakarta.validation.constraints.NotBlank;
import jakarta.validation.constraints.Positive;
import lombok.RequiredArgsConstructor;
import lombok.extern.slf4j.Slf4j;
import org.springframework.http.ResponseEntity;
import org.springframework.security.access.prepost.PreAuthorize;
import org.springframework.web.bind.annotation.DeleteMapping;
import org.springframework.web.bind.annotation.PathVariable;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.RequestBody;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;

import java.util.UUID;

/**
 * Interbank OTC opcione operacije (PR_32 Phase 12/15, Tim 2 §15).
 *
 * <p>Phase 15 update: paths refaktorisani da nose negotiationId u URL-u
 * ({@code /options/{negotiationId}/...}) — usaglaseno sa Phase 5 client kontraktom.
 *
 * <ul>
 *   <li>{@code POST /internal/interbank/options/{negotiationId}/reserve} —
 *       rezervise akcije za option ugovor cija se druga strana nalazi u
 *       stranoj banci.</li>
 *   <li>{@code POST /internal/interbank/options/{negotiationId}/exercise} —
 *       kupac iz strane banke vrsi exercise; commit-ujemo rezervaciju.</li>
 *   <li>{@code DELETE /internal/interbank/options/{negotiationId}/release} —
 *       ugovor je istekao ili odustao; oslobadjamo rezervaciju.</li>
 * </ul>
 *
 * <p>Minimum-viable implementacija (PR_32): negotiationId-to-reservation
 * mapping je in-memory ConcurrentHashMap. Posto je trading-service single
 * instance u nasoj banci, ovo je validno za sad. Prebacivanje na DB tabelu
 * (interbank_option_reservations) je PR_33 kandidat ako mapping mora preziveti
 * restart.
 *
 * <p>Autorizacija: {@code hasRole('SERVICE')}.
 */
@Slf4j
@RestController
@RequestMapping("/internal/interbank/options")
@PreAuthorize("hasRole('SERVICE')")
@RequiredArgsConstructor
public class InterbankOptionController {

    private final InterbankStockReservationService reservationService;

    /** negotiationId -> reservationId UUID. */
    private final java.util.concurrent.ConcurrentHashMap<String, UUID> negotiationToReservation =
            new java.util.concurrent.ConcurrentHashMap<>();

    public record ReserveOptionReq(
            @NotBlank String sellerForeignId,
            @NotBlank String ticker,
            @Positive int quantity
    ) {}

    @PostMapping("/{negotiationId}/reserve")
    public ResponseEntity<Void> reserveOption(
            @PathVariable String negotiationId,
            @RequestBody ReserveOptionReq req) {
        Long sellerUserId = parseUserId(req.sellerForeignId());
        // Koristi negotiationId kao transactionIdLocal za rezervaciju, routing 0
        // (intra-protocol option lifecycle koraci nisu deo TX 2PC routing protokola).
        UUID reservationId = reservationService.reserveStock(
                sellerUserId,
                req.ticker(),
                req.quantity(),
                0,
                negotiationId);
        negotiationToReservation.put(negotiationId, reservationId);
        log.info("Interbank reserveOption: negotiation={} seller={} ticker={} qty={} resId={}",
                negotiationId, sellerUserId, req.ticker(), req.quantity(), reservationId);
        return ResponseEntity.noContent().build();
    }

    @PostMapping("/{negotiationId}/exercise")
    public ResponseEntity<Void> exerciseOption(@PathVariable String negotiationId) {
        UUID reservationId = negotiationToReservation.get(negotiationId);
        if (reservationId == null) {
            log.warn("Interbank exerciseOption: negotiation {} unknown — no-op", negotiationId);
            return ResponseEntity.noContent().build();
        }
        reservationService.commitStock(reservationId);
        negotiationToReservation.remove(negotiationId);
        return ResponseEntity.noContent().build();
    }

    @DeleteMapping("/{negotiationId}/release")
    public ResponseEntity<Void> releaseOption(@PathVariable String negotiationId) {
        UUID reservationId = negotiationToReservation.get(negotiationId);
        if (reservationId == null) {
            log.warn("Interbank releaseOption: negotiation {} unknown — no-op", negotiationId);
            return ResponseEntity.noContent().build();
        }
        reservationService.releaseStock(reservationId);
        negotiationToReservation.remove(negotiationId);
        return ResponseEntity.noContent().build();
    }

    private Long parseUserId(String foreignId) {
        // Per Tim 2 §3.2 spec, foreign-bank-id koristi prefiks "C-" za klijente i
        // "E-" za zaposlene (analogno za njihovu stranu). Trading-service interni
        // userId je goli Long, pa strpamo prefiks pre parsiranja.
        if (foreignId == null) {
            throw new IllegalArgumentException("sellerForeignId must not be null");
        }
        String numericPart = foreignId;
        if (foreignId.startsWith("C-") || foreignId.startsWith("E-")) {
            numericPart = foreignId.substring(2);
        }
        try {
            return Long.parseLong(numericPart);
        } catch (NumberFormatException e) {
            throw new IllegalArgumentException(
                    "Invalid sellerForeignId, expected numeric or 'C-N'/'E-N' format: " + foreignId);
        }
    }
}
