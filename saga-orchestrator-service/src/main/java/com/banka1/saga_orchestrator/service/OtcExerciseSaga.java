package com.banka1.saga_orchestrator.service;

import com.banka1.saga_orchestrator.client.BankingCoreClient;
import com.banka1.saga_orchestrator.client.MarketServiceClient;
import com.banka1.saga_orchestrator.domain.SagaInstance;
import com.banka1.saga_orchestrator.domain.SagaState;
import com.banka1.saga_orchestrator.domain.SagaType;
import com.banka1.saga_orchestrator.repository.SagaInstanceRepository;
import com.fasterxml.jackson.databind.ObjectMapper;
import lombok.RequiredArgsConstructor;
import lombok.extern.slf4j.Slf4j;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;

import java.math.BigDecimal;
import java.util.LinkedHashMap;
import java.util.Map;

/**
 * SAGA OTC_EXERCISE — 5-step distributed transaction kada kupac iskoristi
 * opcioni ugovor (PR_11 C11.4 real implementacija; zameni stub iz PR_04 C4.11).
 *
 * <p>Spec (Celina 4.txt, "Izvrsenje kupoprodaje - SAGA pattern"):
 * <ol>
 *   <li>Rezervacija sredstava na racunu Kupca (banking-core).
 *   <li>Provera/rezervacija hartija u posedstvu Prodavca (market-service).
 *   <li>Transfer sredstava sa Kupca na Prodavca (banking-core).
 *   <li>Transfer vlasnistva nad hartijama na Kupca (market-service).
 *   <li>Final consistency check.
 * </ol>
 *
 * <p>Svaka faza moze fail-ovati. U slucaju failure-a u step-u N, izvrsava se
 * kompenzacija svih prethodnih step-ova (N-1, N-2, ..., 1) u obrnutom redosledu.
 * Kompenzacioni log se cuva u {@code SagaInstance.compensationLog} (jsonb).
 *
 * <p>Idempotency: pre svake saga-e proverava se {@code findBySagaTypeAndCorrelationId}.
 * Ako vec postoji u terminal state-u, preskoci. Ako u IN_PROGRESS-u, prosledi handler-u
 * koji moze ili da nastavi (timeout) ili da odbije (drugi node aktivan).
 */
@Slf4j
@Service
@RequiredArgsConstructor
public class OtcExerciseSaga {

    private final SagaInstanceRepository sagaRepo;
    private final BankingCoreClient banking;
    private final MarketServiceClient market;
    private final ObjectMapper objectMapper;

    @SuppressWarnings("unchecked")
    @Transactional
    public void run(Map<String, Object> event) {
        String correlationId = String.valueOf(event.get("contractId"));

        SagaInstance existing = sagaRepo.findBySagaTypeAndCorrelationId(SagaType.OTC_EXERCISE, correlationId).orElse(null);
        if (existing != null && existing.isFinalState()) {
            log.info("OTC_EXERCISE saga {} vec u terminal state-u {} — preskocenam", correlationId, existing.getState());
            return;
        }
        if (existing != null && existing.getState() == SagaState.IN_PROGRESS) {
            log.warn("OTC_EXERCISE saga {} jos uvek IN_PROGRESS — verovatno duplicate event; preskocenam", correlationId);
            return;
        }

        SagaInstance saga = (existing != null) ? existing : initialize(correlationId, event);
        saga.setState(SagaState.IN_PROGRESS);
        sagaRepo.save(saga);

        Long buyerId = ((Number) event.get("buyerId")).longValue();
        Long sellerId = ((Number) event.get("sellerId")).longValue();
        String stockTicker = (String) event.get("stockTicker");
        Integer amount = ((Number) event.get("amount")).intValue();
        BigDecimal pricePerStock = new BigDecimal(String.valueOf(event.get("pricePerStock")));
        BigDecimal totalCost = pricePerStock.multiply(BigDecimal.valueOf(amount));

        Map<String, Object> compensationLog = new LinkedHashMap<>();
        if (saga.getCompensationLog() instanceof Map<?, ?> existingLog) {
            compensationLog.putAll((Map<String, Object>) existingLog);
        }

        try {
            saga.setCurrentStep(1);
            BankingCoreClient.ReservationResult reservation = banking.reserveFunds(buyerId, totalCost, correlationId);
            compensationLog.put("step1_fundsReservationId", reservation.reservationId());
            log.info("OTC_EXERCISE saga {} step 1 OK ({})", correlationId, reservation.reservationId());

            saga.setCurrentStep(2);
            MarketServiceClient.StockReservationResult stockRes = market.reserveStocks(sellerId, stockTicker, amount, correlationId);
            compensationLog.put("step2_stocksReservationId", stockRes.reservationId());
            log.info("OTC_EXERCISE saga {} step 2 OK ({})", correlationId, stockRes.reservationId());

            saga.setCurrentStep(3);
            String buyerAccount = banking.resolveDefaultAccountNumber(buyerId);
            String sellerAccount = banking.resolveDefaultAccountNumber(sellerId);
            BankingCoreClient.TransferResult transfer = banking.internalTransfer(buyerAccount, sellerAccount, totalCost, correlationId);
            compensationLog.put("step3_transferId", transfer.transferId());
            log.info("OTC_EXERCISE saga {} step 3 OK ({})", correlationId, transfer.transferId());

            saga.setCurrentStep(4);
            MarketServiceClient.OwnershipTransferResult ownership = market.transferOwnership(stockRes.reservationId(), buyerId, correlationId);
            compensationLog.put("step4_ownershipTransferId", ownership.ownershipTransferId());
            log.info("OTC_EXERCISE saga {} step 4 OK ({})", correlationId, ownership.ownershipTransferId());

            saga.setCurrentStep(5);
            if (compensationLog.size() != 4) {
                throw new IllegalStateException("Step 5 inconsistency check failed; compensation log keys=" + compensationLog.keySet());
            }
            log.info("OTC_EXERCISE saga {} step 5 OK (final check)", correlationId);

            saga.setState(SagaState.COMPLETED);
            saga.setCompensationLog(compensationLog);
            sagaRepo.save(saga);
        } catch (Exception ex) {
            log.error("OTC_EXERCISE saga {} failed in step {}: {}", correlationId, saga.getCurrentStep(), ex.toString());
            saga.setCompensationLog(compensationLog);
            compensate(saga, compensationLog, correlationId, ex);
        }
    }

    /**
     * Kompenzacija u obrnutom redosledu izvrsenih step-ova. Idempotentno — ako
     * neki REST poziv fail-uje pri rollback-u, log error ali nastavi sa preostalim
     * compensation step-ovima da bi se sistem doveo do konzistentnog stanja.
     */
    private void compensate(SagaInstance saga, Map<String, Object> compensationLog, String correlationId, Exception cause) {
        saga.setState(SagaState.COMPENSATING);
        sagaRepo.save(saga);

        if (compensationLog.containsKey("step4_ownershipTransferId")) {
            try {
                market.reverseOwnership((String) compensationLog.get("step4_ownershipTransferId"), correlationId);
            } catch (Exception e) {
                log.error("Compensation step 4 reverseOwnership FAILED for saga {}: {}", correlationId, e.toString());
            }
        }
        if (compensationLog.containsKey("step3_transferId")) {
            try {
                banking.reverseTransfer((String) compensationLog.get("step3_transferId"), correlationId);
            } catch (Exception e) {
                log.error("Compensation step 3 reverseTransfer FAILED for saga {}: {}", correlationId, e.toString());
            }
        }
        if (compensationLog.containsKey("step2_stocksReservationId")) {
            try {
                market.releaseStocks((String) compensationLog.get("step2_stocksReservationId"), correlationId);
            } catch (Exception e) {
                log.error("Compensation step 2 releaseStocks FAILED for saga {}: {}", correlationId, e.toString());
            }
        }
        if (compensationLog.containsKey("step1_fundsReservationId")) {
            try {
                banking.releaseFunds((String) compensationLog.get("step1_fundsReservationId"), correlationId);
            } catch (Exception e) {
                log.error("Compensation step 1 releaseFunds FAILED for saga {}: {}", correlationId, e.toString());
            }
        }

        saga.setState(SagaState.FAILED);
        Map<String, Object> failureMeta = new LinkedHashMap<>(compensationLog);
        failureMeta.put("failureReason", cause.getMessage() != null ? cause.getMessage() : cause.getClass().getName());
        failureMeta.put("failedAtStep", saga.getCurrentStep());
        saga.setCompensationLog(failureMeta);
        sagaRepo.save(saga);
    }

    private SagaInstance initialize(String correlationId, Map<String, Object> event) {
        SagaInstance saga = new SagaInstance();
        saga.setSagaType(SagaType.OTC_EXERCISE);
        saga.setCorrelationId(correlationId);
        saga.setTotalSteps(SagaType.OTC_EXERCISE.getTotalSteps());
        saga.setCurrentStep(0);
        saga.setState(SagaState.STARTED);
        saga.setPayload(event);
        return saga;
    }
}
