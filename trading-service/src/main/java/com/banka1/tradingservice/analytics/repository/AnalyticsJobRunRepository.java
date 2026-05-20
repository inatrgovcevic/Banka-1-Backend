package com.banka1.tradingservice.analytics.repository;

import com.banka1.tradingservice.analytics.domain.AnalyticsJobRun;
import org.springframework.data.jpa.repository.JpaRepository;

import java.util.Optional;

public interface AnalyticsJobRunRepository extends JpaRepository<AnalyticsJobRun, String> {

    Optional<AnalyticsJobRun> findFirstByStatusOrderByCompletedAtDesc(String status);
}
