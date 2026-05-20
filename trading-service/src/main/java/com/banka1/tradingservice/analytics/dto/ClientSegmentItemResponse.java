package com.banka1.tradingservice.analytics.dto;

import java.math.BigDecimal;

public record ClientSegmentItemResponse(
        Long userId,
        Integer clusterId,
        String segmentLabel,
        BigDecimal totalPortfolioValue,
        BigDecimal totalCostBasis,
        BigDecimal unrealizedPnl,
        Integer holdingsCount,
        BigDecimal maxHoldingPercent,
        Integer orderCount,
        BigDecimal averageOrderValue,
        BigDecimal buySellRatio,
        BigDecimal riskScore
) {
}
