package com.banka1.bankingcore.account.controller.internal;

import com.banka1.account_service.domain.Account;
import com.banka1.account_service.domain.enums.CurrencyCode;
import com.banka1.account_service.repository.AccountRepository;
import lombok.RequiredArgsConstructor;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.*;

import java.util.Map;

/**
 * REST endpoint-i koje SAGA orchestrator poziva za interne account operacije.
 * Koristi AccountRepository direktno — bez HTTP roundtrip-a ka samom sebi.
 */
@RestController
@RequestMapping("/accounts/internal")
@RequiredArgsConstructor
public class InternalAccountController {

    private final AccountRepository accountRepository;

    @GetMapping("/default/{ownerId}")
    public ResponseEntity<Map<String, String>> defaultAccount(@PathVariable Long ownerId) {
        Account account = accountRepository.findByVlasnikAndCurrencyCode(ownerId, CurrencyCode.RSD)
                .orElseThrow(() -> new IllegalStateException(
                        "Klijent " + ownerId + " nema RSD tekuci racun."));
        return ResponseEntity.ok(Map.of("ownerId", String.valueOf(ownerId), "accountNumber", account.getBrojRacuna()));
    }
}
