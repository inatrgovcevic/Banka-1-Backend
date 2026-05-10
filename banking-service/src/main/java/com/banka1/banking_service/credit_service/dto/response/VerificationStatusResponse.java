package com.banka1.banking_service.credit_service.dto.response;

import lombok.Getter;
import lombok.NoArgsConstructor;
import lombok.Setter;

/**
 * DTO containing verification status information for a verification session.
 * Used to check if a user's identity has been verified through KYC process.
 */
@Getter
@Setter
@NoArgsConstructor
public class VerificationStatusResponse {
    /** The ID of the verification session. */
    private Long sessionId;

    /** The status of the verification (VERIFIED, PENDING, REJECTED, etc.). */
    private String status;

    /**
     * Checks if the verification status is VERIFIED.
     *
     * @return true if status equals "VERIFIED", false otherwise
     */
    public boolean isVerified() {
        return "VERIFIED".equals(status);
    }
}
