package com.banka1.tradingservice.analytics.repository;

import com.banka1.tradingservice.analytics.domain.AnalyticsClientSegment;
import org.springframework.data.jpa.repository.JpaRepository;

import java.util.List;

public interface AnalyticsClientSegmentRepository extends JpaRepository<AnalyticsClientSegment, Long> {

    List<AnalyticsClientSegment> findAllByRunIdOrderByRiskScoreDescUserIdAsc(String runId);
}
