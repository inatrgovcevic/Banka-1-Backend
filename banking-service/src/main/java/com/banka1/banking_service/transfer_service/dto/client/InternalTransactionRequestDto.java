package com.banka1.banking_service.transfer_service.dto.client;

import java.math.BigDecimal;

/**
 * Zahtev za internu transakciju (legacy ili specifične svrhe).
 */
public record InternalTransactionRequestDto(
        BigDecimal amount,
        String referenceType,
        String referenceId
) {}