package com.banka1.banking_service.transfer_service.controller;

import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RestController;

/**
 * Kontroler zadužen za proveru statusa dostupnosti servisa (Health Check).
 */

@RestController
public class HealthController {
    /**
     * Jednostavan endpoint koji potvrđuje da je servis aktivan i zaštićen.
     * @return poruka o statusu servisa
     */
    @GetMapping("/hello")
    public String hello() {
        return "Transfer service is UP and SECURED!";
    }
}