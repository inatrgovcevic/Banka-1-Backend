package com.banka1.tradingservice.analytics.dto;

import java.time.LocalDateTime;

public record AnalyticsRunResponse(
        String runId,
        String jobName,
        String status,
        LocalDateTime startedAt,
        LocalDateTime completedAt,
        String message
) {
}
