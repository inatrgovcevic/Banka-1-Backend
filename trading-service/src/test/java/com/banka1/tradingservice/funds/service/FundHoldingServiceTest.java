package com.banka1.tradingservice.funds.service;

import com.banka1.tradingservice.funds.domain.FundHolding;
import com.banka1.tradingservice.funds.repository.FundHoldingRepository;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.ArgumentCaptor;
import org.mockito.InjectMocks;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;

import java.math.BigDecimal;
import java.util.Optional;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

/**
 * Unit testovi za PR_14 C14.7 + PR_15 C15.6 — FundHolding lifecycle.
 */
@ExtendWith(MockitoExtension.class)
class FundHoldingServiceTest {

    @Mock private FundHoldingRepository repository;

    @InjectMocks private FundHoldingService service;

    @Test
    void addOrUpdate_kreiraNovHolding_kadFundJosNemaTicker() {
        when(repository.findByFundIdAndStockTickerAndDeletedFalse(1L, "AAPL"))
                .thenReturn(Optional.empty());
        when(repository.save(any(FundHolding.class))).thenAnswer(inv -> inv.getArgument(0));

        FundHolding result = service.addOrUpdate(1L, "AAPL", 100, new BigDecimal("150.00"));

        assertThat(result.getFundId()).isEqualTo(1L);
        assertThat(result.getStockTicker()).isEqualTo("AAPL");
        assertThat(result.getQuantity()).isEqualTo(100);
        assertThat(result.getAvgUnitPrice()).isEqualByComparingTo("150.0000");
        assertThat(result.isDeleted()).isFalse();
    }

    @Test
    void addOrUpdate_uvecava_postojeci_holding_sa_weighted_avg() {
        FundHolding existing = FundHolding.builder()
                .id(10L).fundId(1L).stockTicker("AAPL")
                .quantity(100).avgUnitPrice(new BigDecimal("150.0000"))
                .deleted(false)
                .build();
        when(repository.findByFundIdAndStockTickerAndDeletedFalse(1L, "AAPL"))
                .thenReturn(Optional.of(existing));
        when(repository.save(any(FundHolding.class))).thenAnswer(inv -> inv.getArgument(0));

        FundHolding result = service.addOrUpdate(1L, "AAPL", 50, new BigDecimal("180.00"));

        // avgNew = (100*150 + 50*180) / 150 = (15000 + 9000) / 150 = 160.00
        assertThat(result.getQuantity()).isEqualTo(150);
        assertThat(result.getAvgUnitPrice()).isEqualByComparingTo("160.0000");
        assertThat(result.getUpdatedAt()).isNotNull();
    }

    @Test
    void addOrUpdate_throws_kadJeAddedQuantityNula_iliNegativna() {
        assertThatThrownBy(() -> service.addOrUpdate(1L, "AAPL", 0, new BigDecimal("150.00")))
                .isInstanceOf(IllegalArgumentException.class);
        assertThatThrownBy(() -> service.addOrUpdate(1L, "AAPL", -5, new BigDecimal("150.00")))
                .isInstanceOf(IllegalArgumentException.class);
        assertThatThrownBy(() -> service.addOrUpdate(1L, "AAPL", 10, BigDecimal.ZERO))
                .isInstanceOf(IllegalArgumentException.class);
    }

    @Test
    void reduce_smanjuje_quantity_ne_oslobadja_red_kad_jos_ima_ostatak() {
        FundHolding existing = FundHolding.builder()
                .id(10L).fundId(1L).stockTicker("AAPL")
                .quantity(100).avgUnitPrice(new BigDecimal("150.0000"))
                .deleted(false)
                .build();
        when(repository.findByFundIdAndStockTickerAndDeletedFalse(1L, "AAPL"))
                .thenReturn(Optional.of(existing));
        when(repository.save(any(FundHolding.class))).thenAnswer(inv -> inv.getArgument(0));

        FundHolding result = service.reduce(1L, "AAPL", 30);

        assertThat(result.getQuantity()).isEqualTo(70);
        assertThat(result.isDeleted()).isFalse();
    }

    @Test
    void reduce_soft_delete_kad_quantity_padne_na_nula() {
        FundHolding existing = FundHolding.builder()
                .id(10L).fundId(1L).stockTicker("AAPL")
                .quantity(100).avgUnitPrice(new BigDecimal("150.0000"))
                .deleted(false)
                .build();
        when(repository.findByFundIdAndStockTickerAndDeletedFalse(1L, "AAPL"))
                .thenReturn(Optional.of(existing));
        when(repository.save(any(FundHolding.class))).thenAnswer(inv -> inv.getArgument(0));

        FundHolding result = service.reduce(1L, "AAPL", 100);

        assertThat(result.getQuantity()).isZero();
        assertThat(result.isDeleted()).isTrue();
    }

    @Test
    void reduce_throws_kad_nema_dovoljno_hartija() {
        FundHolding existing = FundHolding.builder()
                .id(10L).fundId(1L).stockTicker("AAPL")
                .quantity(50).avgUnitPrice(new BigDecimal("150.0000"))
                .deleted(false)
                .build();
        when(repository.findByFundIdAndStockTickerAndDeletedFalse(1L, "AAPL"))
                .thenReturn(Optional.of(existing));

        assertThatThrownBy(() -> service.reduce(1L, "AAPL", 100))
                .isInstanceOf(IllegalStateException.class)
                .hasMessageContaining("Nedovoljno");
    }

    @Test
    void reduce_throws_kad_holding_ne_postoji() {
        when(repository.findByFundIdAndStockTickerAndDeletedFalse(1L, "AAPL"))
                .thenReturn(Optional.empty());

        assertThatThrownBy(() -> service.reduce(1L, "AAPL", 10))
                .isInstanceOf(IllegalStateException.class)
                .hasMessageContaining("ne poseduje");
    }

    @Test
    void addOrUpdate_zaokruzuje_avgUnitPrice_na_4_decimale() {
        when(repository.findByFundIdAndStockTickerAndDeletedFalse(1L, "AAPL"))
                .thenReturn(Optional.empty());
        ArgumentCaptor<FundHolding> captor = ArgumentCaptor.forClass(FundHolding.class);
        when(repository.save(captor.capture())).thenAnswer(inv -> inv.getArgument(0));

        service.addOrUpdate(1L, "AAPL", 33, new BigDecimal("100.123456789"));

        assertThat(captor.getValue().getAvgUnitPrice().scale()).isEqualTo(4);
    }
}
