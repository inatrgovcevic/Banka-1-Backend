package com.banka1.saga_orchestrator.service;

import com.banka1.saga_orchestrator.client.BankingCoreClient;
import com.banka1.saga_orchestrator.client.TradingServiceClient;
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
 * 2-step saga za isplatu kada likvidnaSredstva nedovoljna (PR_11 C11.6 real).
 *
 * <p>Step 1: market-service likvidira hartije fonda dok ne pokrije iznos.
 * Step 2: banking-core transfer fond -> klijent.
 *
 * <p>Failure rollback je delicate: ako Step 1 uspe a Step 2 fail-uje, hartije
 * ostaju "prodate" (cena trzista promenjena u medjuvremenu). Idealno bi bilo
 * "buy back", ali to bi bio gubitak na razlici cene. Pragmaticno: alert na admin,
 * manualna intervencija; saga.compensationLog beleži liquidationId i transferId.
 */
@Slf4j
@Service
@RequiredArgsConstructor
public class FundRedeemWithLiquidationSaga {

    private final SagaInstanceRepository sagaRepo;
    private final BankingCoreClient banking;
    /** PR_15 C15.3: prebaceno sa MarketServiceClient na TradingServiceClient
     *  jer FundHolding zivi u trading-service-u. */
    private final TradingServiceClient trading;
    private final RabbitTemplate rabbitTemplate;

    @Value("${saga.rabbit.exchange:saga.exchange}")
    private String exchange;

    @Transactional
    public void run(Map<String, Object> event) {
        String correlationId = String.valueOf(event.get("transactionId"));

        SagaInstance existing = sagaRepo.findBySagaTypeAndCorrelationId(SagaType.FUND_LIQUIDATION_FOR_REDEMPTION, correlationId).orElse(null);
        if (existing != null && existing.isFinalState()) {
            log.info("FUND_LIQUIDATION saga {} vec u {} — preskocenam", correlationId, existing.getState());
            return;
        }

        SagaInstance saga = (existing != null) ? existing : initialize(correlationId, event);
        saga.setState(SagaState.IN_PROGRESS);
        sagaRepo.save(saga);

        Long fundId = ((Number) event.get("fundId")).longValue();
        BigDecimal amount = new BigDecimal(String.valueOf(event.get("amount")));
        String fromAcc = (String) event.get("fundAccountNumber");
        String toAcc = (String) event.get("toAccountNumber");

        Map<String, Object> log_ = new LinkedHashMap<>();
        try {
            saga.setCurrentStep(1);
            TradingServiceClient.LiquidationResult liq = trading.liquidateForFund(fundId, amount, correlationId);
            log_.put("step1_liquidationId", liq.liquidationId());
            log_.put("step1_liquidatedAmount", liq.liquidatedAmount());
            log_.put("step1_holdingsSold", liq.holdingsSold());
            log.info("FUND_LIQUIDATION saga {} step 1 OK (liquidated {})", correlationId, liq.liquidatedAmount());

            saga.setCurrentStep(2);
            BankingCoreClient.TransferResult tr = banking.internalTransfer(fromAcc, toAcc, amount, correlationId);
            log_.put("step2_transferId", tr.transferId());
            log.info("FUND_LIQUIDATION saga {} step 2 OK (transfer {})", correlationId, tr.transferId());

            saga.setCompensationLog(log_);
            saga.setState(SagaState.COMPLETED);
            sagaRepo.save(saga);

            Map<String, Object> completedEvent = new LinkedHashMap<>(event);
            completedEvent.put("transferId", tr.transferId());
            completedEvent.put("liquidationId", liq.liquidationId());
            rabbitTemplate.convertAndSend(exchange, "saga.FUND_LIQUIDATION_FOR_REDEMPTION.STEP_2.fund.success", completedEvent);
        } catch (Exception ex) {
            log.error("FUND_LIQUIDATION saga {} FAILED in step {}: {}", correlationId, saga.getCurrentStep(), ex.toString());
            log_.put("failureReason", ex.getMessage() != null ? ex.getMessage() : ex.getClass().getName());
            log_.put("failedAtStep", saga.getCurrentStep());
            if (log_.containsKey("step1_liquidationId") && saga.getCurrentStep() == 2) {
                log_.put("alertRequired", "Liquidation completed but transfer failed — manual intervention needed");
            }
            saga.setCompensationLog(log_);
            saga.setState(SagaState.FAILED);
            sagaRepo.save(saga);

            Map<String, Object> failureEvent = new LinkedHashMap<>(event);
            failureEvent.put("failureReason", log_.get("failureReason"));
            rabbitTemplate.convertAndSend(exchange, "saga.FUND_LIQUIDATION_FOR_REDEMPTION.STEP_X.fund.failure", failureEvent);
        }
    }

    private SagaInstance initialize(String correlationId, Map<String, Object> event) {
        SagaInstance saga = new SagaInstance();
        saga.setSagaType(SagaType.FUND_LIQUIDATION_FOR_REDEMPTION);
        saga.setCorrelationId(correlationId);
        saga.setTotalSteps(SagaType.FUND_LIQUIDATION_FOR_REDEMPTION.getTotalSteps());
        saga.setCurrentStep(0);
        saga.setState(SagaState.STARTED);
        saga.setPayload(event);
        return saga;
    }
}
