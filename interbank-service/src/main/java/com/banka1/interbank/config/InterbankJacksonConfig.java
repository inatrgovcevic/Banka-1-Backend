package com.banka1.interbank.config;

import org.springframework.boot.jackson.autoconfigure.JsonMapperBuilderCustomizer;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import tools.jackson.core.StreamWriteFeature;

/**
 * PR_32 Phase 4: Jackson 3.x kompatibilan customizer koji enable-uje
 * {@link StreamWriteFeature#WRITE_BIGDECIMAL_AS_PLAIN}.
 *
 * <p>Razlog: u Jackson 2.x ova osobina je bila u
 * {@code SerializationFeature.WRITE_BIGDECIMAL_AS_PLAIN} i moglo se podesiti
 * kroz {@code spring.jackson.serialization.write-bigdecimal-as-plain=true}.
 * U Jackson 3.x premestena je u {@code StreamWriteFeature}, koji Spring Boot 4
 * vise ne expose-uje kroz {@code spring.jackson.*} namespace, pa je potreban
 * eksplicitan customizer bean.
 *
 * <p>Tim 2 §12.1 zahteva da BigDecimal vrednosti u JSON payload-u budu zapisane
 * "kao obican broj bez exponent notacije" — bez {@code WRITE_BIGDECIMAL_AS_PLAIN}
 * Jackson 3 default-no enkodira veoma velike/male BigDecimal-e u scientific
 * notation, sto interpartner banka moze odbiti.
 */
@Configuration
public class InterbankJacksonConfig {

    @Bean
    public JsonMapperBuilderCustomizer writeBigDecimalAsPlainCustomizer() {
        return builder -> builder.configure(StreamWriteFeature.WRITE_BIGDECIMAL_AS_PLAIN, true);
    }
}
