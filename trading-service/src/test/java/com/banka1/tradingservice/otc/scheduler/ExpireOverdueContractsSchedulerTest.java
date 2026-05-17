package com.banka1.tradingservice.otc.scheduler;

import com.banka1.tradingservice.otc.domain.OptionContract;
import com.banka1.tradingservice.otc.domain.OptionContractStatus;
import com.banka1.tradingservice.otc.repository.OptionContractRepository;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.ArgumentCaptor;
import org.mockito.InjectMocks;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;

import java.math.BigDecimal;
import java.time.LocalDate;
import java.util.List;

import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.eq;
import static org.mockito.Mockito.never;
import static org.mockito.Mockito.times;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

/**
 * PR_32 Phase 12 KRIT #4: verifikuje da cron expiruje ACTIVE ugovore kojima je
 * settlementDate prosao.
 */
@ExtendWith(MockitoExtension.class)
class ExpireOverdueContractsSchedulerTest {

    @Mock private OptionContractRepository contractRepo;

    @InjectMocks private ExpireOverdueContractsScheduler scheduler;

    @Test
    void expireOverdueContracts_flipujeAktivneIstekle() {
        OptionContract c1 = newContract(1L, LocalDate.now().minusDays(1));
        OptionContract c2 = newContract(2L, LocalDate.now().minusDays(5));

        when(contractRepo.findByStatusAndSettlementDateBefore(eq(OptionContractStatus.ACTIVE), any(LocalDate.class)))
                .thenReturn(List.of(c1, c2));

        scheduler.expireOverdueContracts();

        ArgumentCaptor<OptionContract> captor = ArgumentCaptor.forClass(OptionContract.class);
        verify(contractRepo, times(2)).save(captor.capture());
        List<OptionContract> saved = captor.getAllValues();
        assertThat(saved).hasSize(2);
        assertThat(saved).allMatch(c -> c.getStatus() == OptionContractStatus.EXPIRED);
    }

    @Test
    void expireOverdueContracts_neRadiNista_kadNemaStaleUgovora() {
        when(contractRepo.findByStatusAndSettlementDateBefore(eq(OptionContractStatus.ACTIVE), any(LocalDate.class)))
                .thenReturn(List.of());

        scheduler.expireOverdueContracts();

        verify(contractRepo, never()).save(any(OptionContract.class));
    }

    private static OptionContract newContract(long id, LocalDate settled) {
        OptionContract c = new OptionContract();
        c.setId(id);
        c.setStockTicker("AAPL");
        c.setBuyerId(100L);
        c.setSellerId(200L);
        c.setAmount(10);
        c.setPricePerStock(new BigDecimal("150"));
        c.setSettlementDate(settled);
        c.setStatus(OptionContractStatus.ACTIVE);
        c.setOfferId(50L);
        return c;
    }
}
