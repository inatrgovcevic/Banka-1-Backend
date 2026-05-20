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
@Table(name = "analytics_top_tickers")
@Getter
@Setter
@NoArgsConstructor
@SuppressWarnings("JpaDataSourceORMInspection")
public class AnalyticsTopTicker {

    @Id
    @GeneratedValue(strategy = GenerationType.IDENTITY)
    private Long id;

    @Column(name = "run_id", length = 36, nullable = false)
    private String runId;

    @Column(name = "ticker_rank", nullable = false)
    private Integer tickerRank;

    @Column(name = "listing_id", nullable = false)
    private Long listingId;

    @Column(name = "ticker", length = 64, nullable = false)
    private String ticker;

    @Column(name = "traded_quantity", nullable = false)
    private Long tradedQuantity;

    @Column(name = "traded_notional", precision = 19, scale = 4, nullable = false)
    private BigDecimal tradedNotional;

    @Column(name = "order_count", nullable = false)
    private Integer orderCount;

    @Column(name = "transaction_count", nullable = false)
    private Integer transactionCount;

    @Column(name = "computed_at", nullable = false)
    private LocalDateTime computedAt;
}
