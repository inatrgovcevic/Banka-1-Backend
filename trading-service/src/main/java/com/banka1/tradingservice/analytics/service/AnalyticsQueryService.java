package com.banka1.tradingservice.analytics.service;

import com.banka1.tradingservice.analytics.domain.AnalyticsClientSegment;
import com.banka1.tradingservice.analytics.domain.AnalyticsJobRun;
import com.banka1.tradingservice.analytics.domain.AnalyticsPortfolioRisk;
import com.banka1.tradingservice.analytics.domain.AnalyticsTopTicker;
import com.banka1.tradingservice.analytics.dto.AnalyticsRunResponse;
import com.banka1.tradingservice.analytics.dto.ClientSegmentItemResponse;
import com.banka1.tradingservice.analytics.dto.ClientSegmentsResponse;
import com.banka1.tradingservice.analytics.dto.PortfolioRiskResponse;
import com.banka1.tradingservice.analytics.dto.TopTickerItemResponse;
import com.banka1.tradingservice.analytics.dto.TopTickersResponse;
import com.banka1.tradingservice.analytics.repository.AnalyticsClientSegmentRepository;
import com.banka1.tradingservice.analytics.repository.AnalyticsJobRunRepository;
import com.banka1.tradingservice.analytics.repository.AnalyticsPortfolioRiskRepository;
import com.banka1.tradingservice.analytics.repository.AnalyticsTopTickerRepository;
import lombok.RequiredArgsConstructor;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;
import org.springframework.web.server.ResponseStatusException;

import java.util.List;
import java.util.Optional;

import static org.springframework.http.HttpStatus.NOT_FOUND;

@Service
@RequiredArgsConstructor
@Transactional(readOnly = true)
public class AnalyticsQueryService {

    private static final String COMPLETED = "COMPLETED";

    private final AnalyticsJobRunRepository analyticsJobRunRepository;
    private final AnalyticsClientSegmentRepository analyticsClientSegmentRepository;
    private final AnalyticsPortfolioRiskRepository analyticsPortfolioRiskRepository;
    private final AnalyticsTopTickerRepository analyticsTopTickerRepository;

    public AnalyticsRunResponse getLatestRun() {
        return latestCompletedRun()
                .map(this::toRunResponse)
                .orElseThrow(() -> new ResponseStatusException(NOT_FOUND, "No completed analytics run exists."));
    }

    public ClientSegmentsResponse getClientSegments() {
        Optional<AnalyticsJobRun> latestRun = latestCompletedRun();
        if (latestRun.isEmpty()) {
            return new ClientSegmentsResponse(null, null, List.of());
        }

        AnalyticsJobRun run = latestRun.get();
        List<ClientSegmentItemResponse> segments = analyticsClientSegmentRepository
                .findAllByRunIdOrderByRiskScoreDescUserIdAsc(run.getRunId())
                .stream()
                .map(this::toClientSegmentResponse)
                .toList();

        return new ClientSegmentsResponse(run.getRunId(), run.getCompletedAt(), segments);
    }

    public PortfolioRiskResponse getPortfolioRisk(Long userId) {
        AnalyticsJobRun run = latestCompletedRun()
                .orElseThrow(() -> new ResponseStatusException(NOT_FOUND, "No completed analytics run exists."));

        return analyticsPortfolioRiskRepository.findByRunIdAndUserId(run.getRunId(), userId)
                .map(risk -> toPortfolioRiskResponse(run, risk))
                .orElseThrow(() -> new ResponseStatusException(
                        NOT_FOUND,
                        "No portfolio risk analytics found for userId=%d.".formatted(userId)
                ));
    }

    public TopTickersResponse getTopTickers() {
        Optional<AnalyticsJobRun> latestRun = latestCompletedRun();
        if (latestRun.isEmpty()) {
            return new TopTickersResponse(null, null, List.of());
        }

        AnalyticsJobRun run = latestRun.get();
        List<TopTickerItemResponse> tickers = analyticsTopTickerRepository
                .findAllByRunIdOrderByTickerRankAsc(run.getRunId())
                .stream()
                .map(this::toTopTickerResponse)
                .toList();

        return new TopTickersResponse(run.getRunId(), run.getCompletedAt(), tickers);
    }

    private Optional<AnalyticsJobRun> latestCompletedRun() {
        return analyticsJobRunRepository.findFirstByStatusOrderByCompletedAtDesc(COMPLETED);
    }

    private AnalyticsRunResponse toRunResponse(AnalyticsJobRun run) {
        return new AnalyticsRunResponse(
                run.getRunId(),
                run.getJobName(),
                run.getStatus(),
                run.getStartedAt(),
                run.getCompletedAt(),
                run.getMessage()
        );
    }

    private ClientSegmentItemResponse toClientSegmentResponse(AnalyticsClientSegment segment) {
        return new ClientSegmentItemResponse(
                segment.getUserId(),
                segment.getClusterId(),
                segment.getSegmentLabel(),
                segment.getTotalPortfolioValue(),
                segment.getTotalCostBasis(),
                segment.getUnrealizedPnl(),
                segment.getHoldingsCount(),
                segment.getMaxHoldingPercent(),
                segment.getOrderCount(),
                segment.getAverageOrderValue(),
                segment.getBuySellRatio(),
                segment.getRiskScore()
        );
    }

    private PortfolioRiskResponse toPortfolioRiskResponse(AnalyticsJobRun run, AnalyticsPortfolioRisk risk) {
        return new PortfolioRiskResponse(
                run.getRunId(),
                run.getCompletedAt(),
                risk.getUserId(),
                risk.getTotalMarketValue(),
                risk.getTotalCostBasis(),
                risk.getUnrealizedPnl(),
                risk.getHoldingsCount(),
                risk.getMaxHoldingPercent(),
                risk.getDiversificationScore(),
                risk.getRiskScore(),
                risk.getRiskLevel()
        );
    }

    private TopTickerItemResponse toTopTickerResponse(AnalyticsTopTicker ticker) {
        return new TopTickerItemResponse(
                ticker.getTickerRank(),
                ticker.getListingId(),
                ticker.getTicker(),
                ticker.getTradedQuantity(),
                ticker.getTradedNotional(),
                ticker.getOrderCount(),
                ticker.getTransactionCount()
        );
    }
}
