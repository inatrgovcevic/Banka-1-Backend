package com.banka1.saga_orchestrator.config;

import com.fasterxml.jackson.databind.ObjectMapper;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;

/**
 * PR_19 C19.X: explicitan ObjectMapper bean.
 *
 * <p>Spring Boot 4.0.3 JacksonAutoConfiguration nije pouzdano stvarao ObjectMapper
 * bean u saga-orchestrator kontekstu (vidi {@code OtcExerciseSaga} koja zahteva
 * ObjectMapper kroz constructor injection). Eksplicitan @Bean garantuje.
 */
@Configuration
public class JacksonConfig {

    @Bean
    public ObjectMapper objectMapper() {
        return new ObjectMapper();
    }
}
