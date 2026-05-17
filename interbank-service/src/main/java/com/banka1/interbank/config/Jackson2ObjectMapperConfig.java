package com.banka1.interbank.config;

import com.fasterxml.jackson.databind.DeserializationFeature;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.SerializationFeature;
import com.fasterxml.jackson.datatype.jsr310.JavaTimeModule;
import java.util.List;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import org.springframework.http.MediaType;
import org.springframework.http.converter.HttpMessageConverter;
import org.springframework.http.converter.json.MappingJackson2HttpMessageConverter;
import org.springframework.web.servlet.config.annotation.WebMvcConfigurer;

/**
 * PR_32 Phase 7: Jackson 2.x {@link ObjectMapper} bean + HTTP message converter za inter-bank
 * servise.
 *
 * <p>Spring Boot 4 / Jackson 3 auto-konfigurise {@code tools.jackson.databind.JsonMapper}
 * (Jackson 3 API) ali NE auto-registruje Jackson 2 {@link ObjectMapper}. Vise klasa u
 * interbank-service-u koristi Jackson 2 API direktno (zbog
 * {@code jackson-datatype-jsr310} 2.21.1 koji uvozimo u {@code build.gradle.kts}):
 * <ul>
 *   <li>{@link com.banka1.interbank.service.TransactionExecutorService} —
 *       {@code @Profile("!test")}, postojeci konstruktor sa {@link ObjectMapper}.</li>
 *   <li>{@link com.banka1.interbank.service.InterbankMessageService} — persist idempotency
 *       cache request body kao JSON string.</li>
 *   <li>{@link com.banka1.interbank.controller.InboundDispatcher} — deserialize
 *       {@link com.fasterxml.jackson.databind.JsonNode} message u tipiziran DTO record.</li>
 * </ul>
 *
 * <p><strong>HTTP boundary problem:</strong> Spring Boot 4 default HTTP message converter
 * pokusava da deserialize-uje {@code @RequestBody InterbankMessagePayload} preko Jackson 3,
 * ali polje {@code message} je tipa {@code com.fasterxml.jackson.databind.JsonNode} (Jackson 2)
 * — Jackson 3 ne zna kako da kreira Jackson 2 JsonNode. Resenje: registrujemo
 * {@link MappingJackson2HttpMessageConverter} sa ovim ObjectMapper-om kao PRVI converter za
 * application/json, da Jackson 2 reader uhvati request pre Jackson 3 reader-a.
 *
 * <p>Bean ObjectMapper ima:
 * <ul>
 *   <li>{@link JavaTimeModule} — {@code OffsetDateTime} / {@code Instant} serijalizacija u ISO-8601.</li>
 *   <li>{@code WRITE_DATES_AS_TIMESTAMPS=false} — datumi kao stringovi, ne kao long.</li>
 *   <li>{@code USE_BIG_DECIMAL_FOR_FLOATS=true} — Tim 2 §12.1 precision invariant.</li>
 * </ul>
 */
@Configuration
public class Jackson2ObjectMapperConfig implements WebMvcConfigurer {

    @Bean
    public ObjectMapper interbankObjectMapper() {
        ObjectMapper mapper = new ObjectMapper();
        mapper.registerModule(new JavaTimeModule());
        mapper.disable(SerializationFeature.WRITE_DATES_AS_TIMESTAMPS);
        mapper.enable(DeserializationFeature.USE_BIG_DECIMAL_FOR_FLOATS);
        return mapper;
    }

    /**
     * HTTP message converter za application/json koji koristi Jackson 2 {@link ObjectMapper}.
     * Mora biti registrovan PRE Jackson 3 default converter-a (Spring registruje extend-ovane
     * konvertere na pocetku liste) tako da deserialize-uje {@code InterbankMessagePayload}
     * (sa Jackson 2 {@code JsonNode} poljem) ispravno.
     */
    @Override
    public void extendMessageConverters(List<HttpMessageConverter<?>> converters) {
        MappingJackson2HttpMessageConverter jackson2Converter = new MappingJackson2HttpMessageConverter(
                interbankObjectMapper());
        jackson2Converter.setSupportedMediaTypes(List.of(MediaType.APPLICATION_JSON));
        converters.add(0, jackson2Converter);
    }
}
