package com.banka1.order.config;

import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;

import java.time.Clock;

/**
 * Centralized time source so date-sensitive services can be tested deterministically.
 */
@Configuration
public class TimeConfig {

    @Bean
    public Clock systemClock() {
        return Clock.systemDefaultZone();
    }
}
