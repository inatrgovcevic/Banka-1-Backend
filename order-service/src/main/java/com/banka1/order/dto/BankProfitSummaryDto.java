package com.banka1.order.dto;

import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Data;
import lombok.NoArgsConstructor;

import java.math.BigDecimal;
import java.time.LocalDateTime;

/**
 * Bank-wide trading P&L summary (PR_17 C17.6).
 *
 * <p>Spec (Celina 4.txt — Portal: Profit Banke): pregled bankine zarade. Trading
 * doprinos je suma komisija preko svih aktuara. Fund doprinos (likvidnaSredstva +
 * vrednost hartija - ukupno ulozeno) sabira se na frontend-u iz {@code GET /funds}
 * jer fond entiteti zive u trading-service-u.
 */
@Data
@Builder
@NoArgsConstructor
@AllArgsConstructor
public class BankProfitSummaryDto {
    /** Suma {@code Transaction.commission} preko svih aktuarskih transakcija. */
    private BigDecimal totalCommission;
    /** Ukupan broj izvrsenih transakcija u intervalu. */
    private Long transactionCount;
    /** Broj distinktnih aktuara koji su imali bar jednu transakciju. */
    private Long distinctActuaries;
    /** Donja granica intervala (null = bez granice). */
    private LocalDateTime from;
    /** Gornja granica intervala (null = bez granice). */
    private LocalDateTime to;
}
