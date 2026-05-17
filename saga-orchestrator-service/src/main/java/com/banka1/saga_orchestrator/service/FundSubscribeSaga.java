package com.banka1.saga_orchestrator.service;

import com.banka1.saga_orchestrator.client.BankingCoreClient;
import com.banka1.saga_orchestrator.domain.SagaInstance;
import com.banka1.saga_orchestrator.domain.SagaState;
import com.banka1.saga_orchestrator.domain.SagaType;
import com.banka1.saga_orchestrator.repository.SagaInstanceRepository;
import lombok.RequiredArgsConstructor;
import lombok.extern.slf4j.Slf4j;
import org.springframework.amqp.rabbit.core.RabbitTemplate;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;

import java.math.BigDecimal;
import java.util.LinkedHashMap;
import java.util.Map;

/**
 * Saga za uplatu klijenta u fond (PR_11 C11.6 real implementacija; zameni stub iz PR_04 C4.12).
 *
 * <p>Single-step: debit klijentovog tekuceg + credit fund accounta. ClientFundPosition
 * azuriranje radi trading-service.InvestmentFundService kada konzumira
 * {@code fund.subscribe.completed} event posle uspesnog transfer-a.
 */
@Slf4j
@Service
@RequiredArgsConstructor
public class FundSubscribeSaga {

    private final SagaInstanceRepository sagaRepo;
    private final BankingCoreClient banking;
    private final RabbitTemplate rabbitTemplate;

    @Value("${saga.rabbit.exchange:saga.exchange}")
    private String exchange;

    @Transactional
    public void run(Map<String, Object> event) {
        String correlationId = String.valueOf(event.get("transactionId"));

        SagaInstance existing = sagaRepo.findBySagaTypeAndCorrelationId(SagaType.FUND_SUBSCRIBE, correlationId).orElse(null);
        if (existing != null && existing.isFinalState()) {
            log.info("FUND_SUBSCRIBE saga {} vec u {} — preskocenam", correlationId, existing.getState());
            return;
        }

        SagaInstance saga = (existing != null) ? existing : initialize(correlationId, event);
        saga.setState(SagaState.IN_PROGRESS);
        sagaRepo.save(saga);

        BigDecimal amount = new BigDecimal(String.valueOf(event.get("amount")));
        String fromAcc = (String) event.get("fromAccountNumber");
        String toAcc = (String) event.get("fundAccountNumber");

        try {
            saga.setCurrentStep(1);
            BankingCoreClient.TransferResult tr = banking.internalTransfer(fromAcc, toAcc, amount, correlationId);

            Map<String, Object> log_ = new LinkedHashMap<>();
            log_.put("step1_transferId", tr.transferId());
            saga.setCompensationLog(log_);
            saga.setState(SagaState.COMPLETED);
            sagaRepo.save(saga);

            // Publishuj completed event tako da trading-service azurira tx.status=COMPLETED i poziciju.
            Map<String, Object> completedEvent = new LinkedHashMap<>(event);
            completedEvent.put("transferId", tr.transferId());
            rabbitTemplate.convertAndSend(exchange, "saga.FUND_SUBSCRIBE.STEP_1.fund.success", completedEvent);

            log.info("FUND_SUBSCRIBE saga {} OK", correlationId);
        } catch (Exception ex) {
            log.error("FUND_SUBSCRIBE saga {} FAILED: {}", correlationId, ex.toString());
            Map<String, Object> failureLog = new LinkedHashMap<>();
            failureLog.put("failureReason", ex.getMessage() != null ? ex.getMessage() : ex.getClass().getName());
            saga.setCompensationLog(failureLog);
            saga.setState(SagaState.FAILED);
            sagaRepo.save(saga);

            // Publishuj failure event tako da trading-service postavi tx.status=FAILED.
            Map<String, Object> failureEvent = new LinkedHashMap<>(event);
            failureEvent.put("failureReason", failureLog.get("failureReason"));
            rabbitTemplate.convertAndSend(exchange, "saga.FUND_SUBSCRIBE.STEP_1.fund.failure", failureEvent);
        }
    }

    private SagaInstance initialize(String correlationId, Map<String, Object> event) {
        SagaInstance saga = new SagaInstance();
        saga.setSagaType(SagaType.FUND_SUBSCRIBE);
        saga.setCorrelationId(correlationId);
        saga.setTotalSteps(SagaType.FUND_SUBSCRIBE.getTotalSteps());
        saga.setCurrentStep(0);
        saga.setState(SagaState.STARTED);
        saga.setPayload(event);
        return saga;
    }
}
