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
@Table(name = "analytics_client_segments")
@Getter
@Setter
@NoArgsConstructor
@SuppressWarnings("JpaDataSourceORMInspection")
public class AnalyticsClientSegment {

    @Id
    @GeneratedValue(strategy = GenerationType.IDENTITY)
    private Long id;

    @Column(name = "run_id", length = 36, nullable = false)
    private String runId;

    @Column(name = "user_id", nullable = false)
    private Long userId;

    @Column(name = "cluster_id", nullable = false)
    private Integer clusterId;

    @Column(name = "segment_label", length = 48, nullable = false)
    private String segmentLabel;

    @Column(name = "total_portfolio_value", precision = 19, scale = 4, nullable = false)
    private BigDecimal totalPortfolioValue;

    @Column(name = "total_cost_basis", precision = 19, scale = 4, nullable = false)
    private BigDecimal totalCostBasis;

    @Column(name = "unrealized_pnl", precision = 19, scale = 4, nullable = false)
    private BigDecimal unrealizedPnl;

    @Column(name = "holdings_count", nullable = false)
    private Integer holdingsCount;

    @Column(name = "max_holding_percent", precision = 9, scale = 4, nullable = false)
    private BigDecimal maxHoldingPercent;

    @Column(name = "order_count", nullable = false)
    private Integer orderCount;

    @Column(name = "average_order_value", precision = 19, scale = 4, nullable = false)
    private BigDecimal averageOrderValue;

    @Column(name = "buy_sell_ratio", precision = 19, scale = 4, nullable = false)
    private BigDecimal buySellRatio;

    @Column(name = "risk_score", precision = 9, scale = 4, nullable = false)
    private BigDecimal riskScore;

    @Column(name = "computed_at", nullable = false)
    private LocalDateTime computedAt;
}
