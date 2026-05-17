package com.banka1.credit_service.config;

import org.springframework.boot.context.properties.ConfigurationProperties;

import java.math.BigDecimal;
import java.util.List;

/**
 * PR_29: Konfigurabilna pravila za izracunavanje kamatne stope kredita.
 *
 * <p>Pre PR_29, magic number-i ({@code -1.5}, {@code 1.5}, {@code 6.25}, {@code 0.25},
 * {@code 0.05}, {@code 1200} i niz iznosa) bili su hardkodirani u
 * {@code LoanServiceImplementation}. Sada se citaju iz application.properties /
 * env vars sa prefiksom {@code banka.loan.interest-rate.*}, sto omogucava
 * podesavanje banke politike bez rebuild-a (npr. ECB referent stopa promena,
 * regulatorna izmena baznice, novi tier iznosa).
 *
 * <p>Default-i odgovaraju ponasanju pre PR_29:
 * <pre>
 *   tiers (RSD): 500_000 / 1M / 2M / 5M / 10M / 20M
 *   base rate: 6.25%
 *   step po tier-u: -0.25% (visi iznos = niza stopa)
 *   overdue increment: +0.05% za OVERDUE status
 *   reference random range: [-1.5, +1.5]%
 *   monthly divisor: 1200 (konvertuje godisnju % u mesecnu decimal)
 * </pre>
 *
 * <p>Spec (Celina 2 sekcija "Krediti"): kamatna stopa zavisi od iznosa kredita
 * (godisnja stopa) i moze biti fiksna ili varijabilna. Kod varijabilne, jednom
 * mesecno se generise novi pomeraj iz [{@link #referenceRandomMin},
 * {@link #referenceRandomMax}].
 */
@ConfigurationProperties(prefix = "banka.loan.interest-rate")
public class LoanInterestRateProperties {

    /** Donja granica za random pomeraj referent stope (default {@code -1.5}). */
    private BigDecimal referenceRandomMin = new BigDecimal("-1.5");

    /** Gornja granica za random pomeraj referent stope (default {@code 1.5}). */
    private BigDecimal referenceRandomMax = new BigDecimal("1.5");

    /**
     * Granice tier-a iznosa kredita (RSD ekvivalent), uzlazno sortirane.
     * Iznos {@code <= tier[i]} dobija {@code base - i * step}; u suprotnom
     * tier {@code i+1}.
     */
    private List<BigDecimal> amountTiers = List.of(
            new BigDecimal("500000"),
            new BigDecimal("1000000"),
            new BigDecimal("2000000"),
            new BigDecimal("5000000"),
            new BigDecimal("10000000"),
            new BigDecimal("20000000")
    );

    /** Bazna godisnja kamatna stopa za prvi tier, izrazena u % (default {@code 6.25}). */
    private BigDecimal baseAnnualRatePercent = new BigDecimal("6.25");

    /** Smanjenje stope po tier-u, izrazeno u % (default {@code 0.25}). */
    private BigDecimal stepPerTierPercent = new BigDecimal("0.25");

    /** Dodatno povecanje stope kada je kredit u OVERDUE statusu, izrazeno u % (default {@code 0.05}). */
    private BigDecimal overdueIncrementPercent = new BigDecimal("0.05");

    /**
     * Delitelj koji konvertuje godisnju kamatnu stopu (u %) u mesecnu decimal
     * stopu pogodnu za PMT formulu (default {@code 1200} = 100 * 12).
     */
    private BigDecimal monthlyDivisor = new BigDecimal("1200");

    public BigDecimal getReferenceRandomMin() { return referenceRandomMin; }
    public void setReferenceRandomMin(BigDecimal referenceRandomMin) { this.referenceRandomMin = referenceRandomMin; }

    public BigDecimal getReferenceRandomMax() { return referenceRandomMax; }
    public void setReferenceRandomMax(BigDecimal referenceRandomMax) { this.referenceRandomMax = referenceRandomMax; }

    public List<BigDecimal> getAmountTiers() { return amountTiers; }
    public void setAmountTiers(List<BigDecimal> amountTiers) { this.amountTiers = amountTiers; }

    public BigDecimal getBaseAnnualRatePercent() { return baseAnnualRatePercent; }
    public void setBaseAnnualRatePercent(BigDecimal baseAnnualRatePercent) { this.baseAnnualRatePercent = baseAnnualRatePercent; }

    public BigDecimal getStepPerTierPercent() { return stepPerTierPercent; }
    public void setStepPerTierPercent(BigDecimal stepPerTierPercent) { this.stepPerTierPercent = stepPerTierPercent; }

    public BigDecimal getOverdueIncrementPercent() { return overdueIncrementPercent; }
    public void setOverdueIncrementPercent(BigDecimal overdueIncrementPercent) { this.overdueIncrementPercent = overdueIncrementPercent; }

    public BigDecimal getMonthlyDivisor() { return monthlyDivisor; }
    public void setMonthlyDivisor(BigDecimal monthlyDivisor) { this.monthlyDivisor = monthlyDivisor; }
}
