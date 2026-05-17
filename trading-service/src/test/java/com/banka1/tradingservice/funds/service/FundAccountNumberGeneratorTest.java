package com.banka1.tradingservice.funds.service;

import org.junit.jupiter.api.Test;

import static org.assertj.core.api.Assertions.assertThat;

class FundAccountNumberGeneratorTest {

    private final FundAccountNumberGenerator gen = new FundAccountNumberGenerator();

    @Test
    void generate_proizvodi_16_cifara() {
        for (int i = 0; i < 100; i++) {
            String acc = gen.generate();
            assertThat(acc).hasSize(16).matches("\\d{16}");
        }
    }

    @Test
    void generate_proizvodi_razlicite_brojeve() {
        String a = gen.generate();
        String b = gen.generate();
        assertThat(a).isNotEqualTo(b);
    }
}
