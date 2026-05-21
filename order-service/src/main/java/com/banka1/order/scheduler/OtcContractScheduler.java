package com.banka1.order.scheduler;

import com.banka1.order.service.OtcNegotiationService;
import lombok.RequiredArgsConstructor;
import org.springframework.scheduling.annotation.Scheduled;
import org.springframework.stereotype.Component;

/**
 * Daily maintenance for accepted OTC contracts.
 */
@Component
@RequiredArgsConstructor
public class OtcContractScheduler {

    private final OtcNegotiationService otcNegotiationService;

    @Scheduled(cron = "${otc.contract.expiration-check-cron:0 0 9 * * *}")
    public void runDailyContractMaintenance() {
        otcNegotiationService.notifyContractsExpiringSoon();
        otcNegotiationService.expireOverdueContracts();
    }
}
