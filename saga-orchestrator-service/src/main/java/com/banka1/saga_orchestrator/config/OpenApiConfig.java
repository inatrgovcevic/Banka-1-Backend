package com.banka1.saga_orchestrator.config;

import io.swagger.v3.oas.models.OpenAPI;
import io.swagger.v3.oas.models.info.Info;
import io.swagger.v3.oas.models.servers.Server;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;

import java.util.List;

@Configuration
public class OpenApiConfig {

    @Bean
    public OpenAPI sagaOpenApi() {
        return new OpenAPI()
                .info(new Info()
                        .title("Saga Orchestrator API")
                        .version("0.0.1")
                        .description(
                                "Distributed transaction orchestration (SAGA pattern) for OTC trade "
                                        + "exercise and investment fund redemption flows. "
                                        + "Admin/internal API only - external traffic must come through api-gateway."
                        ))
                .servers(List.of(
                        new Server().url("/").description("Same-origin via api-gateway"),
                        new Server().url("http://localhost:8095").description("Direct (developer)")
                ));
    }
}
