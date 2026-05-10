package com.banka1.banking_service.transfer_service.dto.client;

import java.math.BigDecimal;

/**
 * Podaci o računu dobavljeni iz Account servisa.
 */
public record AccountDto(
        String accountNumber,  // Broj računa
        Long ownerId,          // ID vlasnika računa
        String currency,       // Valuta računa (npr. RSD, EUR)
        BigDecimal availableBalance, // Raspoloživa sredstva
        String status,         // Status (npr. ACTIVE)
        String accountType     // Tip (npr. LIČNI, POSLOVNI)
) {}
