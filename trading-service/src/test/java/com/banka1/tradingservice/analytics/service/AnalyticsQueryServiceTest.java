package com.banka1.tradingservice.analytics.service;

import com.banka1.tradingservice.analytics.domain.AnalyticsClientSegment;
import com.banka1.tradingservice.analytics.domain.AnalyticsJobRun;
import com.banka1.tradingservice.analytics.domain.AnalyticsPortfolioRisk;
import com.banka1.tradingservice.analytics.domain.AnalyticsTopTicker;
import com.banka1.tradingservice.analytics.dto.ClientSegmentsResponse;
import com.banka1.tradingservice.analytics.dto.PortfolioRiskResponse;
import com.banka1.tradingservice.analytics.dto.TopTickersResponse;
import com.banka1.tradingservice.analytics.repository.AnalyticsClientSegmentRepository;
import com.banka1.tradingservice.analytics.repository.AnalyticsJobRunRepository;
import com.banka1.tradingservice.analytics.repository.AnalyticsPortfolioRiskRepository;
import com.banka1.tradingservice.analytics.repository.AnalyticsTopTickerRepository;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;
import org.springframework.web.server.ResponseStatusException;

import java.math.BigDecimal;
import java.time.LocalDateTime;
import java.util.List;
import java.util.Optional;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;
import static org.mockito.Mockito.when;

@ExtendWith(MockitoExtension.class)
class AnalyticsQueryServiceTest {

    @Mock
    private AnalyticsJobRunRepository analyticsJobRunRepository;

    @Mock
    private AnalyticsClientSegmentRepository analyticsClientSegmentRepository;

    @Mock
    private AnalyticsPortfolioRiskRepository analyticsPortfolioRiskRepository;

    @Mock
    private AnalyticsTopTickerRepository analyticsTopTickerRepository;

    @Test
    void getClientSegmentsReturnsEmptyResponseWhenNoCompletedRunExists() {
        when(analyticsJobRunRepository.findFirstByStatusOrderByCompletedAtDesc("COMPLETED"))
                .thenReturn(Optional.empty());

        ClientSegmentsResponse response = service().getClientSegments();

        assertThat(response.runId()).isNull();
        assertThat(response.computedAt()).isNull();
        assertThat(response.segments()).isEmpty();
    }

    @Test
    void getClientSegmentsReturnsLatestRunSegments() {
        AnalyticsJobRun run = run("run-1", LocalDateTime.of(2026, 5, 20, 10, 0));
        AnalyticsClientSegment segment = new AnalyticsClientSegment();
        segment.setUserId(42L);
        segment.setClusterId(2);
        segment.setSegmentLabel("HIGH_EXPOSURE_TRADER");
        segment.setTotalPortfolioValue(new BigDecimal("100000.0000"));
        segment.setTotalCostBasis(new BigDecimal("80000.0000"));
        segment.setUnrealizedPnl(new BigDecimal("20000.0000"));
        segment.setHoldingsCount(4);
        segment.setMaxHoldingPercent(new BigDecimal("45.0000"));
        segment.setOrderCount(12);
        segment.setAverageOrderValue(new BigDecimal("5500.0000"));
        segment.setBuySellRatio(new BigDecimal("2.0000"));
        segment.setRiskScore(new BigDecimal("52.2500"));

        when(analyticsJobRunRepository.findFirstByStatusOrderByCompletedAtDesc("COMPLETED"))
                .thenReturn(Optional.of(run));
        when(analyticsClientSegmentRepository.findAllByRunIdOrderByRiskScoreDescUserIdAsc("run-1"))
                .thenReturn(List.of(segment));

        ClientSegmentsResponse response = service().getClientSegments();

        assertThat(response.runId()).isEqualTo("run-1");
        assertThat(response.computedAt()).isEqualTo(LocalDateTime.of(2026, 5, 20, 10, 0));
        assertThat(response.segments()).hasSize(1);
        assertThat(response.segments().getFirst().userId()).isEqualTo(42L);
        assertThat(response.segments().getFirst().segmentLabel()).isEqualTo("HIGH_EXPOSURE_TRADER");
    }

    @Test
    void getPortfolioRiskReturnsUserSpecificRiskFromLatestRun() {
        AnalyticsJobRun run = run("run-2", LocalDateTime.of(2026, 5, 20, 11, 0));
        AnalyticsPortfolioRisk risk = new AnalyticsPortfolioRisk();
        risk.setUserId(7L);
        risk.setTotalMarketValue(new BigDecimal("25000.0000"));
        risk.setTotalCostBasis(new BigDecimal("22000.0000"));
        risk.setUnrealizedPnl(new BigDecimal("3000.0000"));
        risk.setHoldingsCount(2);
        risk.setMaxHoldingPercent(new BigDecimal("80.0000"));
        risk.setDiversificationScore(new BigDecimal("8.0000"));
        risk.setRiskScore(new BigDecimal("84.2000"));
        risk.setRiskLevel("HIGH");

        when(analyticsJobRunRepository.findFirstByStatusOrderByCompletedAtDesc("COMPLETED"))
                .thenReturn(Optional.of(run));
        when(analyticsPortfolioRiskRepository.findByRunIdAndUserId("run-2", 7L))
                .thenReturn(Optional.of(risk));

        PortfolioRiskResponse response = service().getPortfolioRisk(7L);

        assertThat(response.runId()).isEqualTo("run-2");
        assertThat(response.userId()).isEqualTo(7L);
        assertThat(response.riskLevel()).isEqualTo("HIGH");
    }

    @Test
    void getPortfolioRiskThrowsWhenUserResultDoesNotExist() {
        AnalyticsJobRun run = run("run-3", LocalDateTime.of(2026, 5, 20, 12, 0));
        when(analyticsJobRunRepository.findFirstByStatusOrderByCompletedAtDesc("COMPLETED"))
                .thenReturn(Optional.of(run));
        when(analyticsPortfolioRiskRepository.findByRunIdAndUserId("run-3", 99L))
                .thenReturn(Optional.empty());

        assertThatThrownBy(() -> service().getPortfolioRisk(99L))
                .isInstanceOf(ResponseStatusException.class)
                .hasMessageContaining("No portfolio risk analytics found");
    }

    @Test
    void getTopTickersReturnsLatestRunTickers() {
        AnalyticsJobRun run = run("run-4", LocalDateTime.of(2026, 5, 20, 13, 0));
        AnalyticsTopTicker ticker = new AnalyticsTopTicker();
        ticker.setTickerRank(1);
        ticker.setListingId(5L);
        ticker.setTicker("AAPL");
        ticker.setTradedQuantity(150L);
        ticker.setTradedNotional(new BigDecimal("30000.0000"));
        ticker.setOrderCount(3);
        ticker.setTransactionCount(2);

        when(analyticsJobRunRepository.findFirstByStatusOrderByCompletedAtDesc("COMPLETED"))
                .thenReturn(Optional.of(run));
        when(analyticsTopTickerRepository.findAllByRunIdOrderByTickerRankAsc("run-4"))
                .thenReturn(List.of(ticker));

        TopTickersResponse response = service().getTopTickers();

        assertThat(response.runId()).isEqualTo("run-4");
        assertThat(response.tickers()).hasSize(1);
        assertThat(response.tickers().getFirst().ticker()).isEqualTo("AAPL");
    }

    private AnalyticsQueryService service() {
        return new AnalyticsQueryService(
                analyticsJobRunRepository,
                analyticsClientSegmentRepository,
                analyticsPortfolioRiskRepository,
                analyticsTopTickerRepository
        );
    }

    private AnalyticsJobRun run(String runId, LocalDateTime completedAt) {
        AnalyticsJobRun run = new AnalyticsJobRun();
        run.setRunId(runId);
        run.setJobName("trading-analytics-spark");
        run.setStatus("COMPLETED");
        run.setStartedAt(completedAt.minusMinutes(2));
        run.setCompletedAt(completedAt);
        run.setMessage("Analytics run completed.");
        return run;
    }
}
