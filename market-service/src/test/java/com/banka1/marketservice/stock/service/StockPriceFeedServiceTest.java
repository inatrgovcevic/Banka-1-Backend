package com.banka1.marketservice.stock.service;

import com.banka1.marketservice.stock.dto.StockPriceSnapshotDto;
import org.junit.jupiter.api.Test;

import java.util.List;

import static org.assertj.core.api.Assertions.assertThat;

class StockPriceFeedServiceTest {

    private final StockPriceFeedService service = new StockPriceFeedService();

    @Test
    void getCurrentPrice_vraca_dto_sa_ticker_om() {
        StockPriceSnapshotDto dto = service.getCurrentPrice("AAPL");
        assertThat(dto).isNotNull();
        assertThat(dto.getTicker()).isEqualTo("AAPL");
        assertThat(dto.getCurrentPrice()).isNotNull();
    }

    @Test
    void getCurrentPrice_keseira_isti_objekat_unutar_15s() {
        StockPriceSnapshotDto first = service.getCurrentPrice("AAPL");
        StockPriceSnapshotDto second = service.getCurrentPrice("AAPL");
        // Cache hit — vraca isti instance referenc
        assertThat(first).isSameAs(second);
    }

    @Test
    void getCurrentPrice_uppercase_ticker_pre_lookup() {
        StockPriceSnapshotDto upper = service.getCurrentPrice("AAPL");
        StockPriceSnapshotDto lower = service.getCurrentPrice("aapl");
        assertThat(upper).isSameAs(lower);
    }

    @Test
    void getCurrentPrices_vraca_listu_za_sve_validne_ticker_e() {
        List<StockPriceSnapshotDto> list = service.getCurrentPrices(List.of("AAPL", "MSFT", "GOOGL"));
        assertThat(list).hasSize(3);
        assertThat(list).extracting(StockPriceSnapshotDto::getTicker).containsExactly("AAPL", "MSFT", "GOOGL");
    }
}
