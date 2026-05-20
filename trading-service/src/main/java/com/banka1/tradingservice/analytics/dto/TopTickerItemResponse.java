package com.banka1.tradingservice.analytics.dto;

import java.math.BigDecimal;

public record TopTickerItemResponse(
        Integer rank,
        Long listingId,
        String ticker,
        Long tradedQuantity,
        BigDecimal tradedNotional,
        Integer orderCount,
        Integer transactionCount
) {
}
