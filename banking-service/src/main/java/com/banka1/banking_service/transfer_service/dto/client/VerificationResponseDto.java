package com.banka1.banking_service.transfer_service.dto.client;

/**
 * Rezultat provere verifikacionog koda.
 */
public record VerificationResponseDto(Long sessionId, String status) {
    public boolean isVerified() {
        return "VERIFIED".equals(status);
    }
}