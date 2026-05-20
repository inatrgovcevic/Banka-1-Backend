package com.banka1.tradingservice.analytics.dto;

import java.math.BigDecimal;
import java.time.LocalDateTime;

public record PortfolioRiskResponse(
        String runId,
        LocalDateTime computedAt,
        Long userId,
        BigDecimal totalMarketValue,
        BigDecimal totalCostBasis,
        BigDecimal unrealizedPnl,
        Integer holdingsCount,
        BigDecimal maxHoldingPercent,
        BigDecimal diversificationScore,
        BigDecimal riskScore,
        String riskLevel
) {
}
