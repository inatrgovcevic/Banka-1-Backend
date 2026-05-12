package com.banka1.saga_orchestrator.listener;

import com.banka1.saga_orchestrator.service.OtcExerciseSaga;
import com.banka1.saga_orchestrator.service.OtcPremiumTransferSaga;
import com.banka1.saga_orchestrator.service.FundSubscribeSaga;
import com.banka1.saga_orchestrator.service.FundRedeemSaga;
import com.banka1.saga_orchestrator.service.FundRedeemWithLiquidationSaga;
import lombok.RequiredArgsConstructor;
import lombok.extern.slf4j.Slf4j;
import org.springframework.amqp.rabbit.annotation.Exchange;
import org.springframework.amqp.rabbit.annotation.Queue;
import org.springframework.amqp.rabbit.annotation.QueueBinding;
import org.springframework.amqp.rabbit.annotation.RabbitListener;
import org.springframework.stereotype.Component;

import java.util.Map;

/**
 * SAGA event listener (PR_04 C4.11). Konzumira sve saga events sa
 * {@code saga.events} topic exchange-a i delegira ka konkretnim saga implementacijama.
 *
 * <p>PR_19 C19.X: svaki listener auto-declare-uje queue + exchange + binding tako da
 * passive declaration ne fail-uje kada saga prvi put startuje na svezem RabbitMQ-u.
 */
@Slf4j
@Component
@RequiredArgsConstructor
public class SagaEventListener {

    private final OtcExerciseSaga otcExerciseSaga;
    private final OtcPremiumTransferSaga otcPremiumTransferSaga;
    private final FundSubscribeSaga fundSubscribeSaga;
    private final FundRedeemSaga fundRedeemSaga;
    private final FundRedeemWithLiquidationSaga fundRedeemWithLiquidationSaga;

    @RabbitListener(
            bindings = @QueueBinding(
                    value = @Queue(value = "saga.otc.premium.queue", durable = "true"),
                    exchange = @Exchange(value = "saga.events", type = "topic", durable = "true"),
                    key = "otc.premium.transfer.requested"))
    public void onOtcPremiumTransfer(Map<String, Object> event) {
        log.info("Received otc.premium.transfer.requested: {}", event);
        otcPremiumTransferSaga.run(event);
    }

    @RabbitListener(
            bindings = @QueueBinding(
                    value = @Queue(value = "saga.otc.exercise.queue", durable = "true"),
                    exchange = @Exchange(value = "saga.events", type = "topic", durable = "true"),
                    key = "otc.exercise.requested"))
    public void onOtcExercise(Map<String, Object> event) {
        log.info("Received otc.exercise.requested: {}", event);
        otcExerciseSaga.run(event);
    }

    @RabbitListener(
            bindings = @QueueBinding(
                    value = @Queue(value = "saga.fund.subscribe.queue", durable = "true"),
                    exchange = @Exchange(value = "saga.events", type = "topic", durable = "true"),
                    key = "fund.subscribe.requested"))
    public void onFundSubscribe(Map<String, Object> event) {
        log.info("Received fund.subscribe.requested: {}", event);
        fundSubscribeSaga.run(event);
    }

    @RabbitListener(
            bindings = @QueueBinding(
                    value = @Queue(value = "saga.fund.redeem.queue", durable = "true"),
                    exchange = @Exchange(value = "saga.events", type = "topic", durable = "true"),
                    key = "fund.redeem.requested"))
    public void onFundRedeem(Map<String, Object> event) {
        log.info("Received fund.redeem.requested: {}", event);
        fundRedeemSaga.run(event);
    }

    @RabbitListener(
            bindings = @QueueBinding(
                    value = @Queue(value = "saga.fund.redeem.with-liquidation.queue", durable = "true"),
                    exchange = @Exchange(value = "saga.events", type = "topic", durable = "true"),
                    key = "fund.redeem.with-liquidation.requested"))
    public void onFundRedeemWithLiquidation(Map<String, Object> event) {
        log.info("Received fund.redeem.with-liquidation.requested: {}", event);
        fundRedeemWithLiquidationSaga.run(event);
    }
}
