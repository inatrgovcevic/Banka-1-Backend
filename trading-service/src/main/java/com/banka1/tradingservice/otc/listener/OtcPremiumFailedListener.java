package com.banka1.tradingservice.otc.listener;

import com.banka1.tradingservice.otc.domain.OptionContract;
import com.banka1.tradingservice.otc.domain.OptionContractStatus;
import com.banka1.tradingservice.otc.repository.OptionContractRepository;
import com.banka1.tradingservice.otc.service.OtcPortfolioService;
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
 * PR_32 Phase 12 KRIT #2: slusa saga OTC_PREMIUM_TRANSFER failed event i
 * flipuje OptionContract iz {@code PENDING_PREMIUM} u {@code CANCELED}.
 *
 * <p>Buyer nije imao dovoljno sredstava (ili je SAGA pala iz drugog razloga).
 * Ugovor se gasi, rezervisane akcije se oslobadjaju.
 */
@Slf4j
@Component
@RequiredArgsConstructor
public class OtcPremiumFailedListener {

    private final OptionContractRepository contractRepo;
    private final OtcPortfolioService portfolioService;

    public record OtcPremiumTransferFailedEvent(Long contractId, String reason) {}

    @RabbitListener(bindings = @QueueBinding(
            value = @Queue(value = "trading.otc.premium.failed", durable = "true"),
            exchange = @Exchange(value = "saga.events", type = ExchangeTypes.TOPIC),
            key = "otc.premium.transfer.failed"
    ))
    @Transactional
    public void onFailed(OtcPremiumTransferFailedEvent event) {
        if (event == null || event.contractId() == null) {
            log.warn("Received empty otc.premium.transfer.failed event — ignoring");
            return;
        }
        contractRepo.findById(event.contractId()).ifPresentOrElse(contract -> {
            if (contract.getStatus() == OptionContractStatus.PENDING_PREMIUM) {
                contract.setStatus(OptionContractStatus.CANCELED);
                contractRepo.save(contract);
                portfolioService.releaseForContract(contract.getSellerId(), contract.getStockTicker(), contract.getAmount());
                log.info("OTC option contract {} CANCELED (premium failed): {}",
                        contract.getId(), event.reason());
            } else {
                log.warn("OTC option contract {} already in status {} — cannot cancel",
                        contract.getId(), contract.getStatus());
            }
        }, () -> log.warn("OTC option contract {} not found — skipping cancel", event.contractId()));
    }

    /** Test-only accessor for the contract status transition logic. */
    void cancel(OptionContract contract) {
        if (contract.getStatus() == OptionContractStatus.PENDING_PREMIUM) {
            contract.setStatus(OptionContractStatus.CANCELED);
            contractRepo.save(contract);
        }
    }
}
