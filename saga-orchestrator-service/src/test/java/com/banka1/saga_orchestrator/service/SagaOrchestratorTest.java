package com.banka1.saga_orchestrator.service;

import com.banka1.saga_orchestrator.domain.SagaInstance;
import com.banka1.saga_orchestrator.domain.SagaState;
import com.banka1.saga_orchestrator.domain.SagaStep;
import com.banka1.saga_orchestrator.domain.SagaType;
import com.banka1.saga_orchestrator.repository.SagaInstanceRepository;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;

import java.util.ArrayList;
import java.util.List;
import java.util.Map;
import java.util.Optional;
import java.util.UUID;

import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.times;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

/**
 * Unit testovi state machine-a po DoD-u Issue #214.
 */
class SagaOrchestratorTest {

    private SagaInstanceRepository repository;
    private RecordingStep step1;
    private RecordingStep step2;
    private SagaOrchestrator orchestrator;

    @BeforeEach
    void setUp() {
        repository = mock(SagaInstanceRepository.class);
        when(repository.save(any(SagaInstance.class))).thenAnswer(inv -> inv.getArgument(0));

        step1 = new RecordingStep(SagaType.OTC_EXERCISE, 0);
        step2 = new RecordingStep(SagaType.OTC_EXERCISE, 1);
        orchestrator = new SagaOrchestrator(repository, List.of(step1, step2));
        orchestrator.registerSteps();
    }

    @Test
    void startSaga_persistsAndExecutesFirstStep() {
        UUID id = orchestrator.startSaga(SagaType.OTC_EXERCISE, Map.of("orderId", 42L));

        assertThat(id).isNotNull();
        assertThat(step1.executeCount).isEqualTo(1);
        assertThat(step2.executeCount).isZero();
        verify(repository, times(2)).save(any(SagaInstance.class)); // STARTED + IN_PROGRESS
    }

    @Test
    void onStepSuccess_advancesToNextStep() {
        SagaInstance instance = SagaInstance.builder()
                .id(UUID.randomUUID())
                .sagaType(SagaType.OTC_EXERCISE)
                .currentStep(0)
                .state(SagaState.IN_PROGRESS)
                .compensationLog(new ArrayList<>())
                .build();
        when(repository.findById(instance.getId())).thenReturn(Optional.of(instance));

        orchestrator.onStepSuccess(instance.getId());

        assertThat(instance.getCurrentStep()).isEqualTo(1);
        assertThat(step2.executeCount).isEqualTo(1);
        assertThat(instance.getState()).isEqualTo(SagaState.IN_PROGRESS);
    }

    @Test
    void allStepsDone_marksCompleted() {
        SagaInstance instance = SagaInstance.builder()
                .id(UUID.randomUUID())
                .sagaType(SagaType.OTC_EXERCISE)
                .currentStep(1)
                .state(SagaState.IN_PROGRESS)
                .compensationLog(new ArrayList<>())
                .build();
        when(repository.findById(instance.getId())).thenReturn(Optional.of(instance));

        orchestrator.onStepSuccess(instance.getId());

        assertThat(instance.getState()).isEqualTo(SagaState.COMPLETED);
        assertThat(instance.getCurrentStep()).isEqualTo(2);
    }

    @Test
    void onStepFailure_compensatesInReverseOrder() {
        SagaInstance instance = SagaInstance.builder()
                .id(UUID.randomUUID())
                .sagaType(SagaType.OTC_EXERCISE)
                .currentStep(2)
                .state(SagaState.IN_PROGRESS)
                .compensationLog(new ArrayList<>())
                .build();
        when(repository.findById(instance.getId())).thenReturn(Optional.of(instance));

        orchestrator.onStepFailure(instance.getId(), "downstream-503");

        assertThat(instance.getState()).isEqualTo(SagaState.FAILED);
        assertThat(step1.compensateCount).isEqualTo(1);
        assertThat(step2.compensateCount).isEqualTo(1);
        // Reverse order: step2 compensated PRE step1
        assertThat(step2.compensateOrder).isLessThan(step1.compensateOrder);
    }

    private static class RecordingStep implements SagaStep {
        private static int compensationOrderCounter = 0;
        private final SagaType type;
        private final int index;
        int executeCount;
        int compensateCount;
        int compensateOrder = -1;

        RecordingStep(SagaType type, int index) {
            this.type = type;
            this.index = index;
        }

        @Override
        public SagaType sagaType() {
            return type;
        }

        @Override
        public int stepIndex() {
            return index;
        }

        @Override
        public void execute(SagaInstance instance) {
            executeCount++;
        }

        @Override
        public void compensate(SagaInstance instance) {
            compensateCount++;
            compensateOrder = compensationOrderCounter++;
        }
    }
}
