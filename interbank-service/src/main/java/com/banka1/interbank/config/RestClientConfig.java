package com.banka1.interbank.config;

import com.banka1.interbank.security.JWTService;
import com.fasterxml.jackson.databind.ObjectMapper;
import java.net.http.HttpClient;
import java.time.Duration;
import java.util.List;
import org.springframework.beans.factory.annotation.Qualifier;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import org.springframework.context.annotation.Profile;
import org.springframework.http.MediaType;
import org.springframework.http.client.ClientHttpRequestInterceptor;
import org.springframework.http.client.JdkClientHttpRequestFactory;
import org.springframework.http.converter.json.MappingJackson2HttpMessageConverter;
import org.springframework.web.client.RestClient;

/**
 * PR_32 Phase 5: konfiguracija {@link RestClient} bean-ova za OUTBOUND HTTP
 * pozive iz interbank-service.
 *
 * <p>Tri interna klijenta ({@code bankingCoreRestClient},
 * {@code tradingRestClient}, {@code userRestClient}) dobijaju shared service
 * JWT interceptor — koristi se kad interbank-service iz async kontekst
 * (RabbitMQ listener / scheduled job / inbound REST handler) zove drugi
 * interni servis bez user JWT-a u SecurityContext-u.
 *
 * <p>Cetvrti bean {@code interbankOutboundRestClient} ne nosi service JWT —
 * partner banke koriste {@code X-Api-Key} header (per Tim 2 §6 spec), pa se
 * Authorization header postavlja ad-hoc u svakom outbound pozivu.
 *
 * <p>{@code @Profile("!test")} iskljucuje konfiguraciju kod unit testova gde
 * Mockito / WireMock obezbedjuju mock RestClient bean-ove.
 */
@Configuration
@Profile("!test")
public class RestClientConfig {

    private final JWTService jwtService;
    private final long tokenValidityMillis;

    public RestClientConfig(JWTService jwtService,
                            @Value("${banka.security.expiration-time:3600000}") long tokenValidityMillis) {
        this.jwtService = jwtService;
        this.tokenValidityMillis = tokenValidityMillis;
    }

    /**
     * Interceptor koji svakom OUTBOUND zahtevu ka internim servisima dodaje
     * {@code Authorization: Bearer <service-jwt>} header. Token se kesira
     * unutar interceptor-a i refreshuje pred istek (vidi
     * {@link ServiceJwtAuthInterceptor}).
     */
    @Bean
    public ClientHttpRequestInterceptor jwtAuthInterceptor() {
        return new ServiceJwtAuthInterceptor(jwtService, tokenValidityMillis);
    }

    /**
     * Shared builder sa zakacenim {@link #jwtAuthInterceptor()} — sve tri
     * RestClient bean-a koji idu ka internim servisima se grade odavde.
     */
    @Bean
    public RestClient.Builder internalRestClientBuilder(ClientHttpRequestInterceptor jwtAuthInterceptor) {
        return RestClient.builder().requestInterceptor(jwtAuthInterceptor);
    }

    /**
     * RestClient ka banking-core-service ({@code /internal/interbank/reserve-monas},
     * {@code .../commit-monas}, {@code .../reservations/{id}},
     * {@code /internal/interbank/account-resolve?num=...}).
     */
    @Bean
    public RestClient bankingCoreRestClient(@Qualifier("internalRestClientBuilder") RestClient.Builder builder,
                                            @Value("${services.banking-core.url}") String baseUrl) {
        return builder.baseUrl(baseUrl).build();
    }

    /**
     * RestClient ka trading-service ({@code /internal/interbank/reserve-stock},
     * {@code .../commit-stock}, {@code .../reservations/{id}},
     * {@code /internal/interbank/options/{id}/(reserve|exercise|release)},
     * {@code /internal/interbank/public-stocks}).
     */
    @Bean
    public RestClient tradingRestClient(@Qualifier("internalRestClientBuilder") RestClient.Builder builder,
                                        @Value("${services.trading.url}") String baseUrl) {
        return builder.baseUrl(baseUrl).build();
    }

    /**
     * RestClient ka user-service ({@code /internal/interbank/user/{type}/{id}}).
     */
    @Bean
    public RestClient userRestClient(@Qualifier("internalRestClientBuilder") RestClient.Builder builder,
                                     @Value("${services.user.url}") String baseUrl) {
        return builder.baseUrl(baseUrl).build();
    }

    /**
     * RestClient za OUTBOUND pozive ka PARTNER bankama (Tim 2 / Banka 2). NE
     * nosi service JWT — partner zahteva {@code X-Api-Key} header koji se
     * setuje ad-hoc po pozivu (outboundToken iz {@link InterbankProperties}).
     *
     * <p>Connect timeout 10s, read timeout 20s — partner banke mogu biti spore
     * pri 2PC commit fazi, ali ne sme da blokira indefinitno.
     */
    @Bean
    public RestClient interbankOutboundRestClient(
            @Qualifier("interbankObjectMapper") ObjectMapper interbankObjectMapper) {
        HttpClient httpClient = HttpClient.newBuilder()
                .connectTimeout(Duration.ofSeconds(10))
                .build();
        JdkClientHttpRequestFactory requestFactory = new JdkClientHttpRequestFactory(httpClient);
        requestFactory.setReadTimeout(Duration.ofSeconds(20));

        // Critical: InterbankMessagePayload sadrzi Jackson 2 JsonNode polje. Default
        // RestClient.builder() bira Jackson 3 message converter koji ne ume da serijalizuje
        // Jackson 2 JsonNode kao nested objekat — fallback je toString() → JSON string literal.
        // Registrujemo Jackson 2 converter ispred default-a da ide kao nested object.
        MappingJackson2HttpMessageConverter jackson2 =
                new MappingJackson2HttpMessageConverter(interbankObjectMapper);
        jackson2.setSupportedMediaTypes(List.of(MediaType.APPLICATION_JSON));

        return RestClient.builder()
                .requestFactory(requestFactory)
                .messageConverters(converters -> converters.add(0, jackson2))
                .build();
    }
}
