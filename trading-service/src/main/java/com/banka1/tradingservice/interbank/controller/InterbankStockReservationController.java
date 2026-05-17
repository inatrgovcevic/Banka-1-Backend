package com.banka1.tradingservice.interbank.controller;

import com.banka1.tradingservice.interbank.service.InterbankStockReservationService;
import jakarta.validation.constraints.NotBlank;
import jakarta.validation.constraints.NotNull;
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
 * REST endpoint-i za interbank 2PC akcionarsku rezervaciju (PR_32 Phase 12).
 *
 * <p>Pozivaju ih interbank-service preko {@code TradingInternalClient}.
 * Autorizacija: {@code hasRole('SERVICE')}.
 *
 * <p>Rute (Phase 5 client kontrakt — Tim 2 §11 / §15):
 * <ul>
 *   <li>{@code POST   /internal/interbank/reserve-stock} — rezervise akcije</li>
 *   <li>{@code POST   /internal/interbank/reservations/{id}/commit-stock} — commit faza</li>
 *   <li>{@code DELETE /internal/interbank/reservations/{id}} — release/abort
 *       (path se ne sukobljava sa banking-core {@code DELETE /reservations/{id}}
 *       jer su servisi razdvojeni; suffix-i {@code commit-stock}/{@code commit-monas}
 *       razlikuju 2PC commit dispatch).</li>
 * </ul>
 */
@Slf4j
@RestController
@RequestMapping("/internal/interbank")
@PreAuthorize("hasRole('SERVICE')")
@RequiredArgsConstructor
public class InterbankStockReservationController {

    private final InterbankStockReservationService reservationService;

    public record ReserveStockReq(
            @NotNull Long sellerUserId,
            @NotBlank String ticker,
            @Positive int quantity,
            int transactionIdRouting,
            @NotBlank String transactionIdLocal
    ) {}

    public record ReserveStockRes(UUID reservationId) {}

    @PostMapping("/reserve-stock")
    public ResponseEntity<ReserveStockRes> reserveStock(@RequestBody ReserveStockReq req) {
        UUID id = reservationService.reserveStock(
                req.sellerUserId(),
                req.ticker(),
                req.quantity(),
                req.transactionIdRouting(),
                req.transactionIdLocal());
        return ResponseEntity.ok(new ReserveStockRes(id));
    }

    @PostMapping("/reservations/{id}/commit-stock")
    public ResponseEntity<Void> commitStock(@PathVariable UUID id) {
        reservationService.commitStock(id);
        return ResponseEntity.noContent().build();
    }

    @DeleteMapping("/reservations/{id}")
    public ResponseEntity<Void> releaseStock(@PathVariable UUID id) {
        reservationService.releaseStock(id);
        return ResponseEntity.noContent().build();
    }
}
