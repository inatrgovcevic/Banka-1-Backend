package com.banka1.saga_orchestrator.domain;

import jakarta.persistence.Column;
import jakarta.persistence.Entity;
import jakarta.persistence.EnumType;
import jakarta.persistence.Enumerated;
import jakarta.persistence.Id;
import jakarta.persistence.PrePersist;
import jakarta.persistence.PreUpdate;
import jakarta.persistence.Table;
import jakarta.persistence.UniqueConstraint;
import jakarta.persistence.Version;
import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Getter;
import lombok.NoArgsConstructor;
import lombok.Setter;
import org.hibernate.annotations.JdbcTypeCode;
import org.hibernate.type.SqlTypes;

import java.time.OffsetDateTime;
import java.util.UUID;

/**
 * Persistentno stanje jedne SAGA instance. Polja po DoD-u Issue #214.
 * payload i compensationLog su JSONB kolone čuvane kao Jackson stabla
 * (Map ili List sa proizvoljnim sadržajem) — orchestrator ih tipuje na
 * runtime kroz {@code ObjectMapper.convertValue}.
 *
 * <p>PR_11 C11.1 dorada: dodato {@code correlationId} polje za idempotency,
 * {@code totalSteps} za multi-step state machine, i {@code @Version} za
 * optimistic locking.
 */
@Entity
@Table(
        name = "saga_instance",
        uniqueConstraints = {
                // PR_11 C11.1: idempotency — (sagaType, correlationId) je prirodni dedup kljuc.
                @UniqueConstraint(
                        name = "uk_saga_type_correlation",
                        columnNames = {"saga_type", "correlation_id"}
                )
        }
)
@Getter
@Setter
@NoArgsConstructor
@AllArgsConstructor
@Builder
public class SagaInstance {

    @Id
    @Column(nullable = false, updatable = false)
    private UUID id;

    @Enumerated(EnumType.STRING)
    @Column(name = "saga_type", nullable = false, length = 64)
    private SagaType sagaType;

    /**
     * PR_11 C11.1: prirodni deduplication kljuc (contractId, transactionId, transferId, sl.).
     * Ako RabbitMQ redelivery pokusa dvostruki upis, unique violation odbija duplikat.
     */
    @Column(name = "correlation_id", nullable = false, length = 64)
    private String correlationId;

    @Column(name = "current_step", nullable = false)
    private int currentStep;

    /** PR_11 C11.1: maksimalan broj step-ova (immutable posle inicijalizacije). */
    @Column(name = "total_steps", nullable = false)
    private int totalSteps;

    @Enumerated(EnumType.STRING)
    @Column(nullable = false, length = 32)
    private SagaState state;

    @JdbcTypeCode(SqlTypes.JSON)
    @Column(columnDefinition = "jsonb")
    private Object payload;

    @JdbcTypeCode(SqlTypes.JSON)
    @Column(name = "compensation_log", columnDefinition = "jsonb")
    private Object compensationLog;

    @Column(name = "created_at", nullable = false, updatable = false)
    private OffsetDateTime createdAt;

    @Column(name = "updated_at", nullable = false)
    private OffsetDateTime updatedAt;

    @Column(name = "retry_count", nullable = false)
    private int retryCount;

    /** PR_11 C11.1: optimistic locking sprecava race kada dva paralelna event-a updatuju isti red. */
    @Version
    @Column(name = "version", nullable = false)
    private Long version;

    @PrePersist
    void prePersist() {
        if (id == null) {
            id = UUID.randomUUID();
        }
        OffsetDateTime now = OffsetDateTime.now();
        if (createdAt == null) {
            createdAt = now;
        }
        updatedAt = now;
        if (state == null) {
            state = SagaState.STARTED;
        }
        // PR_11: auto-popuna totalSteps iz SagaType definicije.
        if (totalSteps == 0 && sagaType != null) {
            totalSteps = sagaType.getTotalSteps();
        }
    }

    @PreUpdate
    void preUpdate() {
        updatedAt = OffsetDateTime.now();
    }

    /** PR_11 C11.1: zatvorena saga prelazi u terminal state (COMPLETED ili FAILED). */
    public boolean isFinalState() {
        return state == SagaState.COMPLETED || state == SagaState.FAILED;
    }
}
