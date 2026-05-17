package com.banka1.exchangeService.controller;

import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RequestHeader;
import org.springframework.web.bind.annotation.RestController;

import java.util.Map;

/**
 * REST controller that provides service health and information endpoints.
 * Returns metadata about the exchange-service including status and gateway configuration.
 */
@RestController
public class ExchangeInfoController {

    /**
     * Returns service health and configuration information.
     *
     * @param forwardedPrefix optional gateway prefix from X-Forwarded-Prefix header,
     *                        defaults to /api/exchange if not present
     * @return map containing service name, status, and gateway prefix information
     */
    // PR_19 C19.X: prefix /exchange/info da bi se izbegao mapping conflict sa
    // StockInfoController-om u konsolidovanom market-service JVM-u.
    @GetMapping("/exchange/info")
    public Map<String, Object> info(@RequestHeader(value = "X-Forwarded-Prefix", required = false) String forwardedPrefix) {
        return Map.of(
                "service", "exchange-service",
                "status", "UP",
                "gatewayPrefix", forwardedPrefix == null ? "/api/exchange" : forwardedPrefix
        );
    }
}
