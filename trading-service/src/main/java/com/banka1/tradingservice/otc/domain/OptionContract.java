package com.banka1.tradingservice.otc.domain;

import jakarta.persistence.*;
import jakarta.validation.constraints.DecimalMin;
import jakarta.validation.constraints.NotBlank;
import jakarta.validation.constraints.NotNull;
import lombok.AllArgsConstructor;
import lombok.Getter;
import lombok.NoArgsConstructor;
import lombok.Setter;

import java.math.BigDecimal;
import java.time.LocalDate;
import java.time.LocalDateTime;

/**
 * Sklopljeni opcioni ugovor — kreira se automatski kada {@link OtcOffer} prilikom
 * dogovora dobija status {@code ACCEPTED} (vidi spec, "2. Postignut dogovor").
 *
 * <p>Status zivotnog ciklusa:
 * <ul>
 *   <li>{@code ACTIVE} — vazeci, kupac moze "Iskoristi" opciju do {@code settlementDate}.
 *   <li>{@code EXERCISED} — kupac iskoristio opciju (SAGA OTC_EXERCISE pokrenuta).
 *   <li>{@code EXPIRED} — settlementDate prosao bez exercise-a; rezervisane akcije
 *       prodavca se oslobadjaju za buduce ugovore.
 * </ul>
 *
 * <p>SAGA OTC_EXERCISE: kada kupac klikne "Iskoristi", saga-orchestrator pokrece:
 *   1. rezervaciju kupcevih sredstava (banking-core),
 *   2. provera/rezervacija prodavcevih akcija (market-service),
 *   3. transfer sredstava ka prodavcu,
 *   4. transfer vlasnistva nad akcijama na kupca,
 *   5. final consistency check.
 * Kompenzacione akcije za sve faze (vidi spec).
 */
@Entity
@Table(
        name = "option_contracts",
        indexes = {
                @Index(name = "idx_option_contracts_buyer_id",        columnList = "buyer_id"),
                @Index(name = "idx_option_contracts_seller_id",       columnList = "seller_id"),
                @Index(name = "idx_option_contracts_status",          columnList = "status"),
                @Index(name = "idx_option_contracts_settlement_date", columnList = "settlement_date")
        }
)
@NoArgsConstructor
@AllArgsConstructor
@Getter
@Setter
public class OptionContract {

    @Id
    @GeneratedValue(strategy = GenerationType.IDENTITY)
    private Long id;

    @NotNull
    @Column(name = "offer_id", nullable = false)
    private Long offerId;

    @NotBlank
    @Column(nullable = false, length = 16)
    private String stockTicker;

    @NotNull
    @Column(name = "buyer_id", nullable = false)
    private Long buyerId;

    @NotNull
    @Column(name = "seller_id", nullable = false)
    private Long sellerId;

    @NotNull
    @DecimalMin(value = "1")
    @Column(nullable = false)
    private Integer amount;

    @NotNull
    @DecimalMin(value = "0.00", inclusive = false)
    @Column(name = "price_per_stock", nullable = false, precision = 19, scale = 2)
    private BigDecimal pricePerStock;

    @NotNull
    @Column(name = "settlement_date", nullable = false)
    private LocalDate settlementDate;

    @NotNull
    @Enumerated(EnumType.STRING)
    @Column(nullable = false, length = 16)
    private OptionContractStatus status;

    @NotNull
    @Column(name = "created_at", nullable = false)
    private LocalDateTime createdAt = LocalDateTime.now();

    @Column(name = "exercised_at")
    private LocalDateTime exercisedAt;

    @Version
    private Long version;
}
