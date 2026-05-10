package com.banka1.banking_service.card_service.swagger;

import io.swagger.v3.oas.models.Components;
import io.swagger.v3.oas.models.OpenAPI;
import io.swagger.v3.oas.models.info.Info;
import io.swagger.v3.oas.models.security.SecurityRequirement;
import io.swagger.v3.oas.models.security.SecurityScheme;
import org.springframework.boot.autoconfigure.condition.ConditionalOnMissingBean;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;

/**
 * OpenAPI (Swagger) configuration for the card-service.
 * Registers the JWT Bearer authentication scheme and API metadata.
 */
@Configuration
public class SwaggerConfig {

    /** Application name displayed in Swagger UI. */
    private static final String APP_TITLE = "Card Service API";

    /** API description displayed in Swagger UI. */
    private static final String APP_DESCRIPTION = "API for card creation and management";

    /** API version. */
    private static final String APP_VERSION = "1.0";

    /**
     * Configures the OpenAPI specification for the service and bearer authentication.
     * Adds the JWT Bearer security scheme and basic API information.
     *
     * @return OpenAPI description of the service
     */
    @Bean
    @ConditionalOnMissingBean(OpenAPI.class)
    public OpenAPI openAPI() {
        return new OpenAPI()
                .addSecurityItem(new SecurityRequirement().addList("bearerAuth"))
                .components(new Components().addSecuritySchemes("bearerAuth",
                        new SecurityScheme()
                                .type(SecurityScheme.Type.HTTP)
                                .scheme("bearer")
                                .bearerFormat("JWT")))
                .info(new Info()
                        .title(APP_TITLE)
                        .description(APP_DESCRIPTION)
                        .version(APP_VERSION));
    }
}
