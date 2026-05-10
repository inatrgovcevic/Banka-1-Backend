package com.banka1.banking_service.card_service.rest_client;

public record VerificationStatusResponse(
        Long sessionId,
        VerificationStatus status
) {
}
