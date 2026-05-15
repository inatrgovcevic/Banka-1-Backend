package com.banka1.tradingservice.otc.controller;

import com.banka1.tradingservice.otc.service.StockReservationService;
import jakarta.validation.constraints.Min;
import jakarta.validation.constraints.NotBlank;
import jakarta.validation.constraints.NotNull;
import lombok.Data;
import lombok.RequiredArgsConstructor;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.*;

import java.util.Map;

/**
 * Internal endpoints za OTC_EXERCISE SAGA — stock reservation i ownership transfer.
 * Poziva ih saga-orchestrator-service direktno (ne ide kroz nginx).
 */
@RestController
@RequestMapping("/stocks/internal")
@RequiredArgsConstructor
public class InternalStockController {

    private final StockReservationService reservationService;

    @PostMapping("/reserve")
    public ResponseEntity<StockReservationService.Reservation> reserve(
            @RequestBody ReserveRequest req,
            @RequestHeader(value = "X-Correlation-Id", required = false) String correlationId) {
        return ResponseEntity.ok(reservationService.reserve(
                req.getOwnerId(), req.getStockTicker(), req.getAmount(),
                correlationId != null ? correlationId : "no-correlation"));
    }

    @DeleteMapping("/reservations/{id}")
    public ResponseEntity<StockReservationService.Reservation> release(
            @PathVariable("id") String reservationId,
            @RequestHeader(value = "X-Correlation-Id", required = false) String correlationId) {
        return ResponseEntity.ok(reservationService.release(reservationId,
                correlationId != null ? correlationId : "no-correlation"));
    }

    @PostMapping("/reservations/{id}/transfer")
    public ResponseEntity<StockReservationService.OwnershipTransfer> transfer(
            @PathVariable("id") String reservationId,
            @RequestBody TransferRequest req,
            @RequestHeader(value = "X-Correlation-Id", required = false) String correlationId) {
        return ResponseEntity.ok(reservationService.transferOwnership(reservationId, req.getBuyerId(),
                correlationId != null ? correlationId : "no-correlation"));
    }

    @PostMapping("/ownership-transfers/{id}/reverse")
    public ResponseEntity<Void> reverseOwnership(
            @PathVariable("id") String ownershipTransferId,
            @RequestHeader(value = "X-Correlation-Id", required = false) String correlationId) {
        reservationService.reverseOwnership(ownershipTransferId,
                correlationId != null ? correlationId : "no-correlation");
        return ResponseEntity.ok().build();
    }

    @Data
    public static class ReserveRequest {
        @NotNull private Long ownerId;
        @NotBlank private String stockTicker;
        @Min(1) private int amount;
    }

    @Data
    public static class TransferRequest {
        @NotNull private Long buyerId;
    }
}