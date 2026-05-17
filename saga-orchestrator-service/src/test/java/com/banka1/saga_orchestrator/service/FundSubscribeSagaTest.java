package com.banka1.saga_orchestrator.service;

import com.banka1.saga_orchestrator.client.BankingCoreClient;
import com.banka1.saga_orchestrator.domain.SagaInstance;
import com.banka1.saga_orchestrator.domain.SagaState;
import com.banka1.saga_orchestrator.domain.SagaType;
import com.banka1.saga_orchestrator.repository.SagaInstanceRepository;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.InjectMocks;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;
import org.springframework.amqp.rabbit.core.RabbitTemplate;
import org.springframework.test.util.ReflectionTestUtils;

import java.util.HashMap;
import java.util.Map;
import java.util.Optional;

import static org.mockito.ArgumentMatchers.*;
import static org.mockito.Mockito.*;

@ExtendWith(MockitoExtension.class)
class FundSubscribeSagaTest {

    @Mock private SagaInstanceRepository sagaRepo;
    @Mock private BankingCoreClient banking;
    @Mock private RabbitTemplate rabbitTemplate;

    @InjectMocks private FundSubscribeSaga saga;

    @Test
    void run_completed_publishuje_success_event() {
        ReflectionTestUtils.setField(saga, "exchange", "saga.exchange");

        Map<String, Object> event = new HashMap<>();
        event.put("transactionId", 99L);
        event.put("clientId", 100L);
        event.put("fundId", 1L);
        event.put("amount", "5000");
        event.put("fromAccountNumber", "CLIENT-ACC");
        event.put("fundAccountNumber", "FUND-ACC");

        when(sagaRepo.findBySagaTypeAndCorrelationId(SagaType.FUND_SUBSCRIBE, "99"))
                .thenReturn(Optional.empty());
        when(sagaRepo.save(any())).thenAnswer(inv -> inv.getArgument(0));
        when(banking.internalTransfer(eq("CLIENT-ACC"), eq("FUND-ACC"), any(), eq("99")))
                .thenReturn(new BankingCoreClient.TransferResult("tx-1", "OK"));

        saga.run(event);

        verify(rabbitTemplate).convertAndSend(eq("saga.exchange"),
                eq("saga.FUND_SUBSCRIBE.STEP_1.fund.success"), any(Object.class));
        verify(sagaRepo, atLeast(1)).save(argThat(s -> s.getState() == SagaState.COMPLETED));
    }

    @Test
    void run_failed_publishuje_failure_event() {
        ReflectionTestUtils.setField(saga, "exchange", "saga.exchange");

        Map<String, Object> event = new HashMap<>();
        event.put("transactionId", 99L);
        event.put("amount", "5000");
        event.put("fromAccountNumber", "CLIENT-ACC");
        event.put("fundAccountNumber", "FUND-ACC");

        when(sagaRepo.findBySagaTypeAndCorrelationId(SagaType.FUND_SUBSCRIBE, "99"))
                .thenReturn(Optional.empty());
        when(sagaRepo.save(any())).thenAnswer(inv -> inv.getArgument(0));
        when(banking.internalTransfer(any(), any(), any(), any()))
                .thenThrow(new RuntimeException("Insufficient"));

        saga.run(event);

        verify(rabbitTemplate).convertAndSend(eq("saga.exchange"),
                eq("saga.FUND_SUBSCRIBE.STEP_1.fund.failure"), any(Object.class));
        verify(sagaRepo, atLeast(1)).save(argThat(s -> s.getState() == SagaState.FAILED));
    }
}
