package com.banka1.order.scheduler;

import com.banka1.order.service.OtcNegotiationService;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;
import org.springframework.scheduling.annotation.Scheduled;

import java.lang.reflect.Method;

import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.Mockito.verify;

@ExtendWith(MockitoExtension.class)
class OtcContractSchedulerTest {

    @Mock
    private OtcNegotiationService otcNegotiationService;

    private OtcContractScheduler scheduler;

    @BeforeEach
    void setUp() {
        scheduler = new OtcContractScheduler(otcNegotiationService);
    }

    @Test
    void runDailyContractMaintenanceDelegatesToService() {
        scheduler.runDailyContractMaintenance();

        verify(otcNegotiationService).notifyContractsExpiringSoon();
        verify(otcNegotiationService).expireOverdueContracts();
    }

    @Test
    void runDailyContractMaintenanceUsesConfigurableCron() throws Exception {
        Method method = OtcContractScheduler.class.getDeclaredMethod("runDailyContractMaintenance");
        Scheduled scheduled = method.getAnnotation(Scheduled.class);

        assertThat(scheduled).isNotNull();
        assertThat(scheduled.cron()).isEqualTo("${otc.contract.expiration-check-cron:0 0 9 * * *}");
    }
}
