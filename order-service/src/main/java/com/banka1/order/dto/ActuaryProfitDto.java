package com.banka1.order.dto;

import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Data;
import lombok.NoArgsConstructor;

import java.math.BigDecimal;

/**
 * Trading profit po aktuaru (PR_14 C14.9).
 *
 * <p>Spec (Celina 4.txt — "Profit Banke / Profit aktuara"): banka zaradjuje od
 * komisija na izvrsenim trgovinama. Ovaj DTO sadrzi:
 * <ul>
 *   <li>{@code userId} — ID aktuara/agenta</li>
 *   <li>{@code totalCommission} — suma {@code Transaction.commission} preko svih
 *       transakcija agenata u zadatom intervalu</li>
 *   <li>{@code transactionCount} — broj izvrsenih transakcija</li>
 * </ul>
 *
 * <p>Mocking sa "Profit aktuara = AUM po fund manageru" (pre PR_14) je domenski
 * netacan — fond menadzeri NISU isti entitet kao trgovacki aktuari, i AUM nije
 * profit. Ovaj DTO menja to na pravi trading P&L.
 */
@Data
@Builder
@NoArgsConstructor
@AllArgsConstructor
public class ActuaryProfitDto {
    private Long userId;
    private BigDecimal totalCommission;
    private Long transactionCount;
    /** PR_15 C15.7: ime zaposlenog za prikaz (join sa employee-service). Null ako lookup ne uspe. */
    private String ime;
    /** PR_15 C15.7: prezime zaposlenog. */
    private String prezime;
    /** PR_15 C15.7: pozicija (AGENT/SUPERVISOR). */
    private String pozicija;
}
