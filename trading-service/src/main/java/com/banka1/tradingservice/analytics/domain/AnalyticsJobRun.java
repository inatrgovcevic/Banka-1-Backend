package com.banka1.tradingservice.analytics.domain;

import jakarta.persistence.Column;
import jakarta.persistence.Entity;
import jakarta.persistence.Id;
import jakarta.persistence.Table;
import lombok.Getter;
import lombok.NoArgsConstructor;
import lombok.Setter;

import java.time.LocalDateTime;

@Entity
@Table(name = "analytics_job_runs")
@Getter
@Setter
@NoArgsConstructor
@SuppressWarnings("JpaDataSourceORMInspection")
public class AnalyticsJobRun {

    @Id
    @Column(name = "run_id", length = 36, nullable = false)
    private String runId;

    @Column(name = "job_name", length = 80, nullable = false)
    private String jobName;

    @Column(name = "status", length = 20, nullable = false)
    private String status;

    @Column(name = "started_at", nullable = false)
    private LocalDateTime startedAt;

    @Column(name = "completed_at")
    private LocalDateTime completedAt;

    @Column(name = "message", length = 512)
    private String message;
}
