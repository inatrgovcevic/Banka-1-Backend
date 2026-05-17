package com.banka1.tradingservice.funds.domain;

import jakarta.persistence.*;
import jakarta.validation.constraints.DecimalMin;
import jakarta.validation.constraints.NotNull;
import lombok.AllArgsConstructor;
import lombok.Getter;
import lombok.NoArgsConstructor;
import lombok.Setter;

import java.math.BigDecimal;
import java.time.LocalDateTime;

/**
 * Pozicija klijenta u fondu (PR_04).
 *
 * <p>Spec (Celina 4.txt, ClientFundPosition):
 * <ul>
 *   <li>{@code totalInvested} — UkupanUlozeniIznos.
 *   <li>{@code procenatFonda} — racuna se izvedeno (totalInvested / fund.value).
 * </ul>
 *
 * <p>Postoji tacno jedan red po (clientId, fundId) paru — UNIQUE constraint.
 */
@Entity
@Table(
        name = "client_fund_positions",
        uniqueConstraints = {
                @UniqueConstraint(name = "uk_client_fund_position_client_fund",
                        columnNames = {"client_id", "fund_id"})
        }
)
@NoArgsConstructor
@AllArgsConstructor
@Getter
@Setter
public class ClientFundPosition {

    @Id
    @GeneratedValue(strategy = GenerationType.IDENTITY)
    private Long id;

    @NotNull
    @Column(name = "client_id", nullable = false)
    private Long clientId;

    @NotNull
    @Column(name = "fund_id", nullable = false)
    private Long fundId;

    @NotNull
    @DecimalMin(value = "0.00")
    @Column(name = "total_invested", nullable = false, precision = 19, scale = 2)
    private BigDecimal totalInvested = BigDecimal.ZERO;

    @NotNull
    @Column(name = "first_invested_at", nullable = false)
    private LocalDateTime firstInvestedAt = LocalDateTime.now();

    @Column(name = "last_modified_at")
    private LocalDateTime lastModifiedAt;

    @Version
    private Long version;
}
