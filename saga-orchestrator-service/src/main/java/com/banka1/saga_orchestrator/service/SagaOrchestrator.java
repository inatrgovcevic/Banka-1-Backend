package com.banka1.saga_orchestrator.service;

import com.banka1.saga_orchestrator.domain.SagaInstance;
import com.banka1.saga_orchestrator.domain.SagaState;
import com.banka1.saga_orchestrator.domain.SagaStep;
import com.banka1.saga_orchestrator.domain.SagaType;
import com.banka1.saga_orchestrator.repository.SagaInstanceRepository;
import jakarta.annotation.PostConstruct;
import lombok.RequiredArgsConstructor;
import lombok.extern.slf4j.Slf4j;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;

import java.util.ArrayList;
import java.util.Comparator;
import java.util.EnumMap;
import java.util.List;
import java.util.Map;
import java.util.UUID;

/**
 * Centralna state-machine za SAGA orkestraciju (Issue #214).
 * <p>
 * Drži registar {@link SagaStep} implementacija po (sagaType, stepIndex) i
 * vodi instance kroz tranzicije: STARTED -&gt; IN_PROGRESS -&gt; COMPLETED ili
 * STARTED -&gt; IN_PROGRESS -&gt; COMPENSATING -&gt; FAILED.
 * <p>
 * Detaljnije implementacije {@link SagaStep}-a (sa pozivima banking/order/otc/fund
 * preko RabbitMQ) dolaze u Issue #220 (OTC) i #231 (fund) — ovaj klas pruža
 * mehaniku, ne konkretne korake.
 */
@Service
@Slf4j
@RequiredArgsConstructor
public class SagaOrchestrator {

    private final SagaInstanceRepository repository;
    private final List<SagaStep> registeredSteps;

    private final Map<SagaType, List<SagaStep>> stepsByType = new EnumMap<>(SagaType.class);

    @PostConstruct
    void registerSteps() {
        for (SagaStep step : registeredSteps) {
            stepsByType
                    .computeIfAbsent(step.sagaType(), t -> new ArrayList<>())
                    .add(step);
        }
        stepsByType.values().forEach(list -> list.sort(Comparator.comparingInt(SagaStep::stepIndex)));
        log.info("SagaOrchestrator registered {} step(s) across {} type(s)",
                registeredSteps.size(), stepsByType.size());
    }

    /**
     * Pokreće novu SAGU. Perzistira instance u STARTED, prelazi u IN_PROGRESS i
     * okida prvi korak.
     *
     * @return id nove SAGA instance
     */
    @Transactional
    public UUID startSaga(SagaType sagaType, Object payload) {
        SagaInstance instance = SagaInstance.builder()
                .sagaType(sagaType)
                .state(SagaState.STARTED)
                .currentStep(0)
                .payload(payload)
                .compensationLog(new ArrayList<>())
                .retryCount(0)
                .build();
        repository.save(instance);
        log.info("Saga {} started (type={})", instance.getId(), sagaType);
        advance(instance);
        return instance.getId();
    }

    /**
     * Naredni korak: pokreće {@link SagaStep#execute(SagaInstance)} ili
     * COMPLETED ako su svi koraci izvršeni.
     */
    @Transactional
    public void advance(SagaInstance instance) {
        List<SagaStep> steps = stepsByType.getOrDefault(instance.getSagaType(), List.of());
        if (instance.getCurrentStep() >= steps.size()) {
            instance.setState(SagaState.COMPLETED);
            repository.save(instance);
            log.info("Saga {} completed", instance.getId());
            return;
        }
        SagaStep step = steps.get(instance.getCurrentStep());
        instance.setState(SagaState.IN_PROGRESS);
        repository.save(instance);
        step.execute(instance);
    }

    /**
     * Hendler success event-a sa servisa: pomera step counter i poziva
     * {@link #advance}.
     */
    @Transactional
    public void onStepSuccess(UUID sagaId) {
        SagaInstance instance = repository.findById(sagaId).orElseThrow();
        instance.setCurrentStep(instance.getCurrentStep() + 1);
        instance.setRetryCount(0);
        repository.save(instance);
        advance(instance);
    }

    /**
     * Hendler failure event-a sa servisa: ulazi u COMPENSATING i kreće da
     * undo-uje sve već završene korake u obrnutom redosledu.
     */
    @Transactional
    public void onStepFailure(UUID sagaId, String reason) {
        SagaInstance instance = repository.findById(sagaId).orElseThrow();
        log.warn("Saga {} step {} failed: {}", sagaId, instance.getCurrentStep(), reason);
        instance.setState(SagaState.COMPENSATING);
        repository.save(instance);
        compensate(instance);
    }

    private void compensate(SagaInstance instance) {
        List<SagaStep> steps = stepsByType.getOrDefault(instance.getSagaType(), List.of());
        for (int i = instance.getCurrentStep() - 1; i >= 0; i--) {
            try {
                steps.get(i).compensate(instance);
            } catch (Exception e) {
                log.error("Compensation step {} for saga {} failed: {}", i, instance.getId(), e.toString());
            }
        }
        instance.setState(SagaState.FAILED);
        repository.save(instance);
    }
}
