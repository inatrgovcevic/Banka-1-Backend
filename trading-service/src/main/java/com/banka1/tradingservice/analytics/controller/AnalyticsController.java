package com.banka1.tradingservice.analytics.controller;

import com.banka1.tradingservice.analytics.dto.AnalyticsRunResponse;
import com.banka1.tradingservice.analytics.dto.ClientSegmentsResponse;
import com.banka1.tradingservice.analytics.dto.PortfolioRiskResponse;
import com.banka1.tradingservice.analytics.dto.TopTickersResponse;
import com.banka1.tradingservice.analytics.service.AnalyticsQueryService;
import lombok.RequiredArgsConstructor;
import org.springframework.security.access.prepost.PreAuthorize;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.PathVariable;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;

@RestController
@RequestMapping("/analytics")
@RequiredArgsConstructor
@PreAuthorize("hasAnyRole('ADMIN', 'SUPERVISOR', 'AGENT', 'SERVICE')")
public class AnalyticsController {

    private final AnalyticsQueryService analyticsQueryService;

    @GetMapping("/runs/latest")
    public AnalyticsRunResponse getLatestRun() {
        return analyticsQueryService.getLatestRun();
    }

    @GetMapping("/clients/segments")
    public ClientSegmentsResponse getClientSegments() {
        return analyticsQueryService.getClientSegments();
    }

    @GetMapping("/users/{userId}/portfolio-risk")
    public PortfolioRiskResponse getPortfolioRisk(@PathVariable Long userId) {
        return analyticsQueryService.getPortfolioRisk(userId);
    }

    @GetMapping("/tickers/top")
    public TopTickersResponse getTopTickers() {
        return analyticsQueryService.getTopTickers();
    }
}
