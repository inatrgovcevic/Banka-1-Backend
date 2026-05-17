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
 * OTC ponuda — opcioni ugovor u pregovorima izmedju kupca i prodavca.
 *
 * <p>Spec (Celina 4.txt, Portal: OTC Ponude i Ugovori):
 * <ul>
 *   <li>{@code stock} — akcija za koju se pregovara (Celina 3 entitet, ali ovde drzimo
 *       kao string ticker da izbegnemo cross-modul FK ka market-service-u).
 *   <li>{@code amount} — kolicina akcija.
 *   <li>{@code pricePerStock} — cena po akciji u valuti akcije.
 *   <li>{@code premium} — cena opcionog ugovora.
 *   <li>{@code settlementDate} — datum isteka opcije.
 *   <li>{@code lastModified}, {@code modifiedBy} — audit za pregovor flow.
 *   <li>{@code status} — PENDING_SELLER | PENDING_BUYER | ACCEPTED | REJECTED | EXPIRED.
 * </ul>
 *
 * <p>Pregovori traju "back and forth" dok jedna strana ne odustane ili dok prodavac ne
 * prihvati. Premiju isplacuje SAGA: kada {@code status = ACCEPTED}, saga-orchestrator
 * inicira premium transfer sa kupcevog na prodavcev racun.
 */
@Entity
@Table(
        name = "otc_offers",
        indexes = {
                @Index(name = "idx_otc_offers_buyer_id",        columnList = "buyer_id"),
                @Index(name = "idx_otc_offers_seller_id",       columnList = "seller_id"),
                @Index(name = "idx_otc_offers_status",          columnList = "status"),
                @Index(name = "idx_otc_offers_settlement_date", columnList = "settlement_date")
        }
)
@NoArgsConstructor
@AllArgsConstructor
@Getter
@Setter
public class OtcOffer {

    @Id
    @GeneratedValue(strategy = GenerationType.IDENTITY)
    private Long id;

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
    @DecimalMin(value = "0.00")
    @Column(nullable = false, precision = 19, scale = 2)
    private BigDecimal premium;

    @NotNull
    @Column(name = "settlement_date", nullable = false)
    private LocalDate settlementDate;

    @NotNull
    @Enumerated(EnumType.STRING)
    @Column(nullable = false, length = 24)
    private OtcOfferStatus status;

    @Column(name = "modified_by", length = 64)
    private String modifiedBy;

    @NotNull
    @Column(name = "last_modified", nullable = false)
    private LocalDateTime lastModified = LocalDateTime.now();

    @NotNull
    @Column(name = "created_at", nullable = false)
    private LocalDateTime createdAt = LocalDateTime.now();

    /** Optimistic locking — sprecava lost update u pregovor flow-u. */
    @Version
    private Long version;

    @PreUpdate
    public void onUpdate() {
        this.lastModified = LocalDateTime.now();
    }
}
