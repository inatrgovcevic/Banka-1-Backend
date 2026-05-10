package com.banka1.banking_service.card_service.dto.card_creation.internal;

import java.math.BigDecimal;

/**
 * DTO for internal calls between services (e.g., account-service).
 * Contains basic account information with English field names
 * for compatibility with the expected format used by other services.
 */
public record InternalAccountDetailsDto(
        String accountNumber,
        Long ownerId,
        String currency,
        BigDecimal availableBalance,
        String status,
        String accountType
) {
}

