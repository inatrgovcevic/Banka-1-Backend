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

@Slf4j
@Component
@RequiredArgsConstructor
public class OtcExerciseCompletedListener {

    private final OptionContractRepository contractRepo;
    private final OtcPortfolioService portfolioService;

    public record OtcExerciseCompletedEvent(Long contractId) {}

    @RabbitListener(bindings = @QueueBinding(
            value = @Queue(value = "trading.otc.exercise.completed", durable = "true"),
            exchange = @Exchange(value = "saga.events", type = ExchangeTypes.TOPIC),
            key = "otc.exercise.completed"
    ))
    @Transactional
    public void onCompleted(OtcExerciseCompletedEvent event) {
        if (event == null || event.contractId() == null) {
            log.warn("Received empty otc.exercise.completed event — ignoring");
            return;
        }
        contractRepo.findById(event.contractId()).ifPresentOrElse(contract -> {
            if (contract.getStatus() == OptionContractStatus.ACTIVE) {
                contract.setStatus(OptionContractStatus.EXERCISED);
                contractRepo.save(contract);
                // Release the accept-time reservation. StockReservationService.transferOwnership()
                // already decremented reservedQuantity for the SAGA's own step-2 reservation, but
                // the original reserveForContract() increment (done at accept time) is separate and
                // must be released here so reservedQuantity doesn't stay permanently inflated.
                portfolioService.releaseForContract(contract.getSellerId(), contract.getStockTicker(), contract.getAmount());
                log.info("OTC contract {} ACTIVE -> EXERCISED, accept-time reservation released", contract.getId());
            } else {
                log.info("OTC contract {} already in status {} — no-op", contract.getId(), contract.getStatus());
            }
        }, () -> log.warn("OTC contract {} not found", event.contractId()));
    }
}