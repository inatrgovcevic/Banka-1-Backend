package com.banka1.saga_orchestrator.scheduled;

import lombok.RequiredArgsConstructor;
import lombok.extern.slf4j.Slf4j;
import net.javacrumbs.shedlock.spring.annotation.SchedulerLock;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.scheduling.annotation.Scheduled;
import org.springframework.stereotype.Component;

/**
 * PR_08 C8.4: cleanup cron za saga_idempotency_log tabelu.
 *
 * <p>PR_06 C6.5 je uveo idempotency log; bez retencije tabela bi rasla bez
 * granica. Spec retention: 14 dana (dovoljno za RabbitMQ message redelivery
 * window i manualne replay scenarije; sve preko toga je yagni).
 *
 * <p>Cron radi svaku noc u 03:00. ShedLock garantuje single-replica izvrsenje.
 */
@Slf4j
@Component
@RequiredArgsConstructor
public class SagaIdempotencyCleanupScheduler {

    private final JdbcTemplate jdbcTemplate;

    @Scheduled(cron = "0 0 3 * * *")
    @SchedulerLock(name = "SagaIdempotencyCleanup", lockAtMostFor = "PT10M")
    public void cleanup() {
        int rows = jdbcTemplate.update(
                "DELETE FROM saga_idempotency_log WHERE processed_at < now() - INTERVAL '14 days'"
        );
        log.info("Saga idempotency cleanup: deleted {} rows older than 14 days", rows);
    }
}
