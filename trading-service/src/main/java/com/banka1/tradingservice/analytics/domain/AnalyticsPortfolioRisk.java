package com.banka1.tradingservice.analytics.domain;

import jakarta.persistence.Column;
import jakarta.persistence.Entity;
import jakarta.persistence.GeneratedValue;
import jakarta.persistence.GenerationType;
import jakarta.persistence.Id;
import jakarta.persistence.Table;
import lombok.Getter;
import lombok.NoArgsConstructor;
import lombok.Setter;

import java.math.BigDecimal;
import java.time.LocalDateTime;

@Entity
@Table(name = "analytics_portfolio_risk")
@Getter
@Setter
@NoArgsConstructor
@SuppressWarnings("JpaDataSourceORMInspection")
public class AnalyticsPortfolioRisk {

    @Id
    @GeneratedValue(strategy = GenerationType.IDENTITY)
    private Long id;

    @Column(name = "run_id", length = 36, nullable = false)
    private String runId;

    @Column(name = "user_id", nullable = false)
    private Long userId;

    @Column(name = "total_market_value", precision = 19, scale = 4, nullable = false)
    private BigDecimal totalMarketValue;

    @Column(name = "total_cost_basis", precision = 19, scale = 4, nullable = false)
    private BigDecimal totalCostBasis;

    @Column(name = "unrealized_pnl", precision = 19, scale = 4, nullable = false)
    private BigDecimal unrealizedPnl;

    @Column(name = "holdings_count", nullable = false)
    private Integer holdingsCount;

    @Column(name = "max_holding_percent", precision = 9, scale = 4, nullable = false)
    private BigDecimal maxHoldingPercent;

    @Column(name = "diversification_score", precision = 9, scale = 4, nullable = false)
    private BigDecimal diversificationScore;

    @Column(name = "risk_score", precision = 9, scale = 4, nullable = false)
    private BigDecimal riskScore;

    @Column(name = "risk_level", length = 16, nullable = false)
    private String riskLevel;

    @Column(name = "computed_at", nullable = false)
    private LocalDateTime computedAt;
}
