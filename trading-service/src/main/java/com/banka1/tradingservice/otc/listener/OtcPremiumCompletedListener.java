package com.banka1.tradingservice.otc.listener;

import com.banka1.tradingservice.otc.domain.OptionContract;
import com.banka1.tradingservice.otc.domain.OptionContractStatus;
import com.banka1.tradingservice.otc.repository.OptionContractRepository;
import lombok.RequiredArgsConstructor;
import lombok.extern.slf4j.Slf4j;
import org.springframework.amqp.core.ExchangeTypes;
import org.springframework.amqp.rabbit.annotation.Exchange;
import org.springframework.amqp.rabbit.annotation.Queue;
import org.springframework.amqp.rabbit.annotation.QueueBinding;
import org.springframework.amqp.rabbit.annotation.RabbitListener;
import org.springframework.stereotype.Component;
import org.springframework.transaction.annotation.Transactional;

/**
 * PR_32 Phase 12 KRIT #2: slusa saga OTC_PREMIUM_TRANSFER completed event
 * i flipuje OptionContract iz {@code PENDING_PREMIUM} u {@code ACTIVE}.
 *
 * <p>Saga publish-uje event na {@code saga.events} exchange (TOPIC) sa
 * routing key {@code otc.premium.transfer.completed}.
 */
@Slf4j
@Component
@RequiredArgsConstructor
public class OtcPremiumCompletedListener {

    private final OptionContractRepository contractRepo;

    public record OtcPremiumTransferCompletedEvent(Long contractId) {}

    @RabbitListener(bindings = @QueueBinding(
            value = @Queue(value = "trading.otc.premium.completed", durable = "true"),
            exchange = @Exchange(value = "saga.events", type = ExchangeTypes.TOPIC),
            key = "otc.premium.transfer.completed"
    ))
    @Transactional
    public void onCompleted(OtcPremiumTransferCompletedEvent event) {
        if (event == null || event.contractId() == null) {
            log.warn("Received empty otc.premium.transfer.completed event — ignoring");
            return;
        }
        contractRepo.findById(event.contractId()).ifPresentOrElse(contract -> {
            if (contract.getStatus() == OptionContractStatus.PENDING_PREMIUM) {
                contract.setStatus(OptionContractStatus.ACTIVE);
                contractRepo.save(contract);
                log.info("OTC option contract {} promoted PENDING_PREMIUM -> ACTIVE", contract.getId());
            } else {
                log.info("OTC option contract {} already in status {} — no-op",
                        contract.getId(), contract.getStatus());
            }
        }, () -> log.warn("OTC option contract {} not found — skipping promote", event.contractId()));
    }

    /** Test-only accessor for the contract status transition logic. */
    void promote(OptionContract contract) {
        if (contract.getStatus() == OptionContractStatus.PENDING_PREMIUM) {
            contract.setStatus(OptionContractStatus.ACTIVE);
            contractRepo.save(contract);
        }
    }
}
