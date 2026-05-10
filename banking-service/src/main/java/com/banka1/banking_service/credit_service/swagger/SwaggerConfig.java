package com.banka1.banking_service.credit_service.swagger;

import io.swagger.v3.oas.models.Components;
import io.swagger.v3.oas.models.OpenAPI;
import io.swagger.v3.oas.models.info.Info;
import io.swagger.v3.oas.models.security.SecurityRequirement;
import io.swagger.v3.oas.models.security.SecurityScheme;
import org.springframework.boot.autoconfigure.condition.ConditionalOnMissingBean;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;

/**
 * Swagger/OpenAPI configuration for the Credit Service API.
 * Configures API documentation with security scheme for JWT Bearer token authentication.
 */
@Configuration
public class SwaggerConfig {

    /**
     * Configures OpenAPI bean with API metadata and security scheme.
     * Defines Bearer JWT authentication scheme for all API endpoints.
     *
     * @return configured OpenAPI instance
     */
    @Bean
    @ConditionalOnMissingBean(OpenAPI.class)
    public OpenAPI openAPI() {
        return new OpenAPI()
                .info(new Info()
                        .title("Account service API")
                        .description("API for accounts")
                        .version("1.0"))
                .addSecurityItem(new SecurityRequirement().addList("bearerAuth"))
                .components(new Components()
                        .addSecuritySchemes("bearerAuth",
                                new SecurityScheme()
                                        .type(SecurityScheme.Type.HTTP)
                                        .scheme("bearer")
                                        .bearerFormat("JWT")));
    }
}
