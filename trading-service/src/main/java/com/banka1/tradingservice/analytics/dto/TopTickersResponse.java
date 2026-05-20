package com.banka1.tradingservice.analytics.dto;

import java.time.LocalDateTime;
import java.util.List;

public record TopTickersResponse(
        String runId,
        LocalDateTime computedAt,
        List<TopTickerItemResponse> tickers
) {
}
