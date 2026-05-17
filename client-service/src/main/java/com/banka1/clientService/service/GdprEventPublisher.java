package com.banka1.clientService.service;

import lombok.RequiredArgsConstructor;
import lombok.extern.slf4j.Slf4j;
import org.springframework.amqp.rabbit.core.RabbitTemplate;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.stereotype.Component;
import org.springframework.transaction.support.TransactionSynchronization;
import org.springframework.transaction.support.TransactionSynchronizationManager;

import java.time.OffsetDateTime;
import java.util.Map;

/**
 * Publisher za GDPR cascade events (PR_11 C11.13 real implementacija).
 *
 * <p>Kada KlijentService oznaci klijenta kao soft-deleted (deleted=true),
 * registruje afterCommit callback koji publishuje
 * {@code gdpr.client.soft-deleted} na exchange {@code gdpr.events}; banking-core
 * listener ga konzumira i bulk-update-uje sve account-e/cards/itd. povezane
 * sa tim klijentom.
 *
 * <p>afterCommit garantuje da event ide TEK posle uspesnog commit-a — ako se
 * soft-delete transakcija rollback-uje, kaskada se ne pokrece.
 */
@Slf4j
@Component
@RequiredArgsConstructor
public class GdprEventPublisher {

    private final RabbitTemplate rabbitTemplate;

    @Value("${gdpr.exchange:gdpr.events}")
    private String exchange;

    public void publishClientSoftDeleted(Long clientId, String reason) {
        Map<String, Object> event = Map.of(
                "eventId", java.util.UUID.randomUUID().toString(),
                "clientId", clientId,
                "reason", reason != null ? reason : "client-requested",
                "occurredAt", OffsetDateTime.now().toString()
        );

        Runnable publisher = () -> {
            rabbitTemplate.convertAndSend(exchange, "gdpr.client.soft-deleted", event);
            log.info("GDPR event published: client={} reason={}", clientId, reason);
        };

        if (TransactionSynchronizationManager.isSynchronizationActive()) {
            TransactionSynchronizationManager.registerSynchronization(new TransactionSynchronization() {
                @Override public void afterCommit() { publisher.run(); }
            });
        } else {
            publisher.run();
        }
    }
}
