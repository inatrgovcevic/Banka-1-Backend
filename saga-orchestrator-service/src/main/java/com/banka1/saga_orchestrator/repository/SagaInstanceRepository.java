package com.banka1.saga_orchestrator.repository;

import com.banka1.saga_orchestrator.domain.SagaInstance;
import com.banka1.saga_orchestrator.domain.SagaState;
import com.banka1.saga_orchestrator.domain.SagaType;
import org.springframework.data.domain.Page;
import org.springframework.data.domain.Pageable;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.Lock;
import org.springframework.data.jpa.repository.Query;
import org.springframework.data.repository.query.Param;
import org.springframework.stereotype.Repository;

import jakarta.persistence.LockModeType;
import java.time.OffsetDateTime;
import java.util.List;
import java.util.Optional;
import java.util.UUID;

@Repository
public interface SagaInstanceRepository extends JpaRepository<SagaInstance, UUID> {

    Page<SagaInstance> findByState(SagaState state, Pageable pageable);

    Page<SagaInstance> findBySagaType(SagaType sagaType, Pageable pageable);

    Page<SagaInstance> findByStateAndSagaType(SagaState state, SagaType sagaType, Pageable pageable);

    List<SagaInstance> findByStateIn(List<SagaState> states);

    /**
     * PR_11 C11.1: idempotency lookup — ako saga sa istim (sagaType, correlationId)
     * vec postoji, RabbitMQ redelivery se preskace.
     */
    Optional<SagaInstance> findBySagaTypeAndCorrelationId(SagaType sagaType, String correlationId);

    /**
     * PR_11 C11.1: pessimistic lock za state machine update koji prati event flow.
     * Sprecava race kada dva paralelna response event-a (npr. step-N-completed +
     * timeout-expired) pokusaju update istog reda.
     */
    @Lock(LockModeType.PESSIMISTIC_WRITE)
    @Query("select s from SagaInstance s where s.id = :id")
    Optional<SagaInstance> findByIdForUpdate(@Param("id") UUID id);

    /**
     * PR_11 C11.1: stuck-saga sweeper — saga-e koje su IN_PROGRESS duze od `cutoff`-a
     * su podlozni eskalaciji (manualni audit + restart).
     */
    List<SagaInstance> findByStateAndCreatedAtBefore(SagaState state, OffsetDateTime cutoff);
}
