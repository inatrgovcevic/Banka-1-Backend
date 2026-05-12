package com.banka1.saga_orchestrator.service;

import com.banka1.saga_orchestrator.client.BankingCoreClient;
import com.banka1.saga_orchestrator.client.MarketServiceClient;
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

import java.math.BigDecimal;
import java.util.HashMap;
import java.util.Map;
import java.util.Optional;

import static org.mockito.ArgumentMatchers.*;
import static org.mockito.Mockito.*;

@ExtendWith(MockitoExtension.class)
class FundRedeemWithLiquidationSagaTest {

    @Mock private SagaInstanceRepository sagaRepo;
    @Mock private BankingCoreClient banking;
    @Mock private MarketServiceClient market;
    @Mock private RabbitTemplate rabbitTemplate;

    @InjectMocks private FundRedeemWithLiquidationSaga saga;

    private Map<String, Object> redeemEvent() {
        Map<String, Object> e = new HashMap<>();
        e.put("transactionId", 99L);
        e.put("fundId", 1L);
        e.put("amount", "10000");
        e.put("fundAccountNumber", "FUND-ACC");
        e.put("toAccountNumber", "CLIENT-ACC");
        return e;
    }

    @Test
    void run_oba_step_uspesna_postavlja_completed() {
        ReflectionTestUtils.setField(saga, "exchange", "saga.exchange");
        when(sagaRepo.findBySagaTypeAndCorrelationId(any(), eq("99"))).thenReturn(Optional.empty());
        when(sagaRepo.save(any())).thenAnswer(inv -> inv.getArgument(0));
        when(market.liquidateForFund(eq(1L), any(), eq("99")))
                .thenReturn(new MarketServiceClient.LiquidationResult("liq-1", new BigDecimal("10000"), 5));
        when(banking.internalTransfer(any(), any(), any(), any()))
                .thenReturn(new BankingCoreClient.TransferResult("tx-1", "OK"));

        saga.run(redeemEvent());

        verify(sagaRepo, atLeast(1)).save(argThat(s -> s.getState() == SagaState.COMPLETED));
        verify(rabbitTemplate).convertAndSend(eq("saga.exchange"),
                eq("saga.FUND_LIQUIDATION_FOR_REDEMPTION.STEP_2.fund.success"), any(Object.class));
    }

    @Test
    void run_failure_u_step_2_postavlja_alertRequired_kada_je_step_1_uspeo() {
        ReflectionTestUtils.setField(saga, "exchange", "saga.exchange");
        when(sagaRepo.findBySagaTypeAndCorrelationId(any(), eq("99"))).thenReturn(Optional.empty());
        when(sagaRepo.save(any())).thenAnswer(inv -> inv.getArgument(0));
        when(market.liquidateForFund(any(), any(), any()))
                .thenReturn(new MarketServiceClient.LiquidationResult("liq-1", new BigDecimal("10000"), 5));
        when(banking.internalTransfer(any(), any(), any(), any()))
                .thenThrow(new RuntimeException("Banking down"));

        saga.run(redeemEvent());

        verify(sagaRepo, atLeast(1)).save(argThat(s ->
                s.getState() == SagaState.FAILED
                        && s.getCompensationLog() != null
                        && s.getCompensationLog().toString().contains("alertRequired")));
    }
}
