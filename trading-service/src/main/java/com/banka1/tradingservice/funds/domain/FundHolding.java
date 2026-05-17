package com.banka1.tradingservice.funds.domain;

import jakarta.persistence.*;
import jakarta.validation.constraints.DecimalMin;
import jakarta.validation.constraints.NotBlank;
import jakarta.validation.constraints.NotNull;
import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Getter;
import lombok.NoArgsConstructor;
import lombok.Setter;

import java.math.BigDecimal;
import java.time.LocalDateTime;

/**
 * Hartija u portfoliju investicionog fonda (PR_14 C14.7).
 *
 * <p>Spec (Celina 4.txt, "Vrednost fonda"):
 * <em>vrednost = likvidna sredstva + suma vrednosti hartija fonda</em>.
 * Pre PR_14 nije postojala ni jedna tabela koja drzi "hartije fonda" — fond
 * je imao samo {@code likvidnaSredstva}. Bez ove tabele {@code vrednostFonda}
 * je uvek bio jednak {@code likvidnaSredstva}, sto pravi spec formulu
 * polovicno tacnom.
 *
 * <p>Avgcena (avg_unit_price) potice od istorije kupovine — kada SAGA
 * FUND_INVEST kupuje hartije za fond, holding red se update-uje:
 * weighted average od (postojeca avg * quantity + nova kupovina cena * dodatak)
 * / total quantity.
 *
 * <p>Pri redempciji (FUND_REDEEM_WITH_LIQUIDATION saga, Issue #231) prodaju
 * se hartije fonda; quantity se smanjuje, a ako padne na 0, holding red se
 * obrisava (soft-delete kroz {@code deleted=true}).
 */
@Entity
@Table(
        name = "fund_holdings",
        uniqueConstraints = {
                @UniqueConstraint(
                        name = "uk_fund_holdings_fund_ticker",
                        columnNames = {"fund_id", "stock_ticker"}
                )
        },
        indexes = {
                @Index(name = "idx_fund_holdings_fund_id",      columnList = "fund_id"),
                @Index(name = "idx_fund_holdings_stock_ticker", columnList = "stock_ticker")
        }
)
@NoArgsConstructor
@AllArgsConstructor
@Builder
@Getter
@Setter
public class FundHolding {

    @Id
    @GeneratedValue(strategy = GenerationType.IDENTITY)
    private Long id;

    @NotNull
    @Column(name = "fund_id", nullable = false)
    private Long fundId;

    @NotBlank
    @Column(name = "stock_ticker", nullable = false, length = 16)
    private String stockTicker;

    @NotNull
    @DecimalMin(value = "0")
    @Column(nullable = false)
    private Integer quantity;

    @NotNull
    @DecimalMin(value = "0.00")
    @Column(name = "avg_unit_price", nullable = false, precision = 19, scale = 4)
    private BigDecimal avgUnitPrice;

    @Column(name = "deleted", nullable = false)
    private boolean deleted = false;

    @Column(name = "created_at", nullable = false)
    private LocalDateTime createdAt = LocalDateTime.now();

    @Column(name = "updated_at")
    private LocalDateTime updatedAt;

    @Version
    private Long version;
}
