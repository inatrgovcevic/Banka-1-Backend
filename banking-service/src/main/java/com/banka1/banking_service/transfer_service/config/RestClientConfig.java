package com.banka1.banking_service.transfer_service.config;

import com.banka1.banking_service.transfer_service.security.JWTService;
import lombok.RequiredArgsConstructor;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import org.springframework.context.annotation.Profile;
import org.springframework.http.HttpRequest;
import org.springframework.http.client.ClientHttpRequestExecution;
import org.springframework.http.client.ClientHttpRequestInterceptor;
import org.springframework.http.client.ClientHttpResponse;
import org.springframework.web.client.RestClient;

import java.io.IOException;

/**
 * Konfiguracija za {@link RestClient} instance koje se koriste za komunikaciju sa eksternim mikroservisima.
 * Uključuje automatsko ubrizgavanje JWT tokena za autentifikaciju između servisa.
 */
@Configuration
@RequiredArgsConstructor
@Profile("!local")
public class RestClientConfig {

    private final JWTService jwtService;

    /**
     * Interceptor koji presreće svaki odlazni HTTP zahtev i dodaje "Authorization: Bearer <token>" zaglavlje.
     * Token se generiše putem {@link JWTService} sa ulogom "SERVICE".
     */
    class JwtAuthInterceptor implements ClientHttpRequestInterceptor {
        @Override
        public ClientHttpResponse intercept(HttpRequest request, byte[] body, ClientHttpRequestExecution execution) throws IOException {
            String token = jwtService.generateJwtToken();
            request.getHeaders().set("Authorization", "Bearer " + token);
            return execution.execute(request, body);
        }
    }

    /**
     * Definiše osnovni builder za RestClient sa ugrađenim JWT presretačem.
     * @return builder koji se koristi za kreiranje specifičnih klijenata
     */
    public RestClient.Builder restClientBuilder() {
        return RestClient.builder()
                .requestInterceptor(new JwtAuthInterceptor());
    }

    /**
     * Klijent za komunikaciju sa Account servisom.
     * @param baseUrl URL učitan iz properties fajla (services.account.url)
     */
    @Bean
    public RestClient accountRestClient(RestClient.Builder builder, @Value("${services.account.url}") String baseUrl) {
        return builder.baseUrl(baseUrl).build();
    }

    /**
     * Klijent za komunikaciju sa Exchange (menjačnica) servisom.
     */
    @Bean
    public RestClient exchangeRestClient(RestClient.Builder builder, @Value("${services.exchange.url}") String baseUrl) {
        return builder.baseUrl(baseUrl).build();
    }
    /**
     * Klijent za komunikaciju sa Verification servisom (2FA).
     */
    @Bean
    public RestClient verificationRestClient(RestClient.Builder builder, @Value("${services.verification.url}") String baseUrl) {
        return builder.baseUrl(baseUrl).build();
    }

    /**
     * Klijent za komunikaciju sa Client servisom.
     */
    @Bean
    public RestClient clientRestClient(RestClient.Builder builder, @Value("${services.client.url}") String baseUrl) {
        return builder.baseUrl(baseUrl).build();
    }
}
