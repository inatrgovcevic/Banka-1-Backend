package com.banka1.tradingservice.analytics.repository;

import com.banka1.tradingservice.analytics.domain.AnalyticsPortfolioRisk;
import org.springframework.data.jpa.repository.JpaRepository;

import java.util.Optional;

public interface AnalyticsPortfolioRiskRepository extends JpaRepository<AnalyticsPortfolioRisk, Long> {

    Optional<AnalyticsPortfolioRisk> findByRunIdAndUserId(String runId, Long userId);
}
