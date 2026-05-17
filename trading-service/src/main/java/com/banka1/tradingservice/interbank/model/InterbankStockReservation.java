package com.banka1.tradingservice.interbank.model;

import jakarta.persistence.Column;
import jakarta.persistence.Entity;
import jakarta.persistence.GeneratedValue;
import jakarta.persistence.GenerationType;
import jakarta.persistence.Id;
import jakarta.persistence.PrePersist;
import jakarta.persistence.Table;
import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Getter;
import lombok.NoArgsConstructor;
import lombok.Setter;

import java.time.Instant;
import java.util.UUID;

/**
 * PR_32 Phase 12: interbank 2PC rezervacija akcija u portfoliu.
 *
 * <p>Pratecu sa {@code portfolio.reserved_quantity}: ova tabela drzi per-rezervaciju
 * red (kljuc = reservation_id UUID), dok {@code portfolio.reserved_quantity}
 * agregira ukupnu kolicinu vezanih jedinica za sve obaveze.
 *
 * <p>Status flow:
 * <ul>
 *   <li>{@code HELD} — interbank-service je rezervisao akcije preko
 *       {@code POST /internal/interbank/reserve-stock}. {@code portfolio.reservedQuantity}
 *       je inkrementovan, ali {@code portfolio.quantity} jos nije skinut.</li>
 *   <li>{@code COMMITTED} — uspesno 2PC commit; sad i {@code portfolio.quantity}
 *       pada (akcije su prebacene na kupca).</li>
 *   <li>{@code RELEASED} — 2PC abort; samo {@code reservedQuantity} se vraca,
 *       {@code quantity} ostaje netaknut.</li>
 * </ul>
 *
 * <p>Idempotentnost: ponovljeni commit/release za rezervaciju u terminal stanju je no-op.
 */
@Entity
@Table(name = "interbank_stock_reservations")
@Getter
@Setter
@NoArgsConstructor
@AllArgsConstructor
@Builder
public class InterbankStockReservation {

    @Id
    @GeneratedValue(strategy = GenerationType.IDENTITY)
    private Long id;

    @Column(name = "reservation_id", nullable = false, unique = true)
    private UUID reservationId;

    @Column(name = "transaction_id_routing", nullable = false)
    private int transactionIdRouting;

    @Column(name = "transaction_id_local", nullable = false, length = 64)
    private String transactionIdLocal;

    @Column(name = "portfolio_id", nullable = false)
    private Long portfolioId;

    @Column(nullable = false, length = 16)
    private String ticker;

    @Column(nullable = false)
    private int quantity;

    /** HELD / COMMITTED / RELEASED. */
    @Column(nullable = false, length = 16)
    private String status;

    @Column(name = "created_at", nullable = false, updatable = false)
    private Instant createdAt;

    @Column(name = "finalized_at")
    private Instant finalizedAt;

    @PrePersist
    void onCreate() {
        if (createdAt == null) {
            createdAt = Instant.now();
        }
    }
}
