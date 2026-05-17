package com.banka1.stock_service.controller;

import com.banka1.stock_service.client.ExchangeServiceClient;
import com.banka1.stock_service.config.ExchangeServiceClientProperties;
import com.banka1.stock_service.config.StockMarketDataProperties;
import com.banka1.stock_service.dto.ExchangeServiceInfoResponse;
import com.banka1.stock_service.dto.StockServiceInfoResponse;
import io.swagger.v3.oas.annotations.Operation;
import lombok.AllArgsConstructor;
import org.springframework.security.access.prepost.PreAuthorize;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RequestHeader;
import org.springframework.web.bind.annotation.RestController;

/**
 * Bootstrap REST endpoints for {@code stock-service}.
 * This controller currently exists to verify that the service is running,
 * that the gateway prefix is propagated correctly, and that inter-service
 * communication toward {@code exchange-service} works.
 */
@RestController
@AllArgsConstructor
public class StockInfoController {

    private final ExchangeServiceClient exchangeServiceClient;
    private final ExchangeServiceClientProperties exchangeServiceClientProperties;
    private final StockMarketDataProperties stockMarketDataProperties;


    /**
     * Returns basic metadata about the service and the active integration configuration.
     *
     * @param forwardedPrefix optional gateway prefix from the request header
     * @return bootstrap info response with the service status and main URL configurations
     */
    // PR_19 C19.X: prefix /stocks/info kako bi se izbegao mapping conflict sa
    // ExchangeInfoController#info() u konsolidovanom market-service JVM-u.
    @Operation(summary = "Get stock service info")
    @GetMapping("/stocks/info")
    public StockServiceInfoResponse info(
            @RequestHeader(value = "X-Forwarded-Prefix", required = false) String forwardedPrefix
    ) {
        return new StockServiceInfoResponse(
                "stock-service",
                "UP",
                forwardedPrefix == null ? "/stock" : forwardedPrefix,
                exchangeServiceClientProperties.baseUrl(),
                stockMarketDataProperties.baseUrl(),
                stockMarketDataProperties.apiKey() != null && !stockMarketDataProperties.apiKey().isBlank()
        );
    }

    /**
     * Forwards the call to the {@code exchange-service} info endpoint.
     * The route is protected so it can serve as a simple verification
     * of the JWT resource server configuration.
     *
     * @return exchange service response
     */
    // PR_19 C19.X: prefix /stocks/exchange/info — staro mapiranje /exchange/info se
    // sad sluzi direktno iz ExchangeInfoController-a u istom JVM-u.
    @Operation(summary = "Get exchange service info (forwarded)")
    @GetMapping("/stocks/exchange/info")
    @PreAuthorize("hasAnyRole('CLIENT_BASIC', 'BASIC', 'AGENT', 'SUPERVISOR', 'ADMIN', 'SERVICE')")
    public ExchangeServiceInfoResponse exchangeInfo() {
        return exchangeServiceClient.getInfo();
    }
}
