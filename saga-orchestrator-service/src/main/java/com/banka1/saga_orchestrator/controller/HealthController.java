package com.banka1.saga_orchestrator.controller;

import io.swagger.v3.oas.annotations.Operation;
import io.swagger.v3.oas.annotations.tags.Tag;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;

import java.util.Map;

/**
 * Domain-level health endpoint za saga-orchestrator-service. Spring Actuator
 * pokriva infrastrukturni health (DB, Rabbit) na /actuator/health; ovaj endpoint
 * je tu da bi /saga/health bio brzi liveness check za api-gateway nezavisno od
 * Actuator-a (i zadovoljava DoD #213 "Health endpoint radi").
 */
@RestController
@RequestMapping("/saga")
@Tag(name = "Health", description = "Saga orchestrator liveness")
public class HealthController {

    @Operation(summary = "Liveness check za saga-orchestrator-service")
    @GetMapping("/health")
    public ResponseEntity<Map<String, String>> health() {
        return ResponseEntity.ok(Map.of(
                "status", "UP",
                "service", "saga-orchestrator-service"
        ));
    }
}
