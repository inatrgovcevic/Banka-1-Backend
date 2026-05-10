package com.banka1.banking_service.transfer_service.config;

import io.swagger.v3.oas.models.Components;
import io.swagger.v3.oas.models.OpenAPI;
import io.swagger.v3.oas.models.info.Info;
import io.swagger.v3.oas.models.security.SecurityRequirement;
import io.swagger.v3.oas.models.security.SecurityScheme;
import org.springframework.boot.autoconfigure.condition.ConditionalOnMissingBean;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;

/**
 * Konfiguracija za OpenAPI (Swagger) dokumentaciju Transfer servisa.
 * Podešava metapodatke API-ja i definiše JWT Security Scheme za testiranje zaštićenih ruta.
 */
@Configuration
public class SwaggerConfig {
    /**
     * Kreira OpenAPI specifikaciju servisa.
     * Definiše Bearer Token autentifikaciju pod nazivom "BearerAuthentication".
     * @return OpenAPI konfiguracioni objekat
     */
    @Bean
    @ConditionalOnMissingBean(OpenAPI.class)
    public OpenAPI transferOpenAPI() {
        return new OpenAPI()
                .info(new Info()
                        .title("Transfer Service API")
                        .description("Servis za prenos sredstava između računa istog klijenta.")
                        .version("1.0.0"))
                .addSecurityItem(new SecurityRequirement().addList("BearerAuthentication"))
                .components(new Components()
                        .addSecuritySchemes("BearerAuthentication",
                                new SecurityScheme()
                                        .type(SecurityScheme.Type.HTTP)
                                        .scheme("bearer")
                                        .bearerFormat("JWT")));
    }
}
