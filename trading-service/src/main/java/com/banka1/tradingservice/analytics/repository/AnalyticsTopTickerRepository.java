package com.banka1.tradingservice.analytics.repository;

import com.banka1.tradingservice.analytics.domain.AnalyticsTopTicker;
import org.springframework.data.jpa.repository.JpaRepository;

import java.util.List;

public interface AnalyticsTopTickerRepository extends JpaRepository<AnalyticsTopTicker, Long> {

    List<AnalyticsTopTicker> findAllByRunIdOrderByTickerRankAsc(String runId);
}
