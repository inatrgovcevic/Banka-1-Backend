package com.banka1.banking_service.transfer_service.client;

import com.banka1.banking_service.transfer_service.dto.client.VerificationResponseDto;
/**
 * Interfejs za komunikaciju sa servisom za 2FA verifikaciju (Verification Service).
 */
public interface VerificationClient {
    /**
     * Šalje zahtev za validaciju jednokratnog koda (OTP) unutar specifične sesije.
     * @param sessionId ID sesije pokrenute za verifikaciju transfera
     * @return DTO sa statusom verifikacije i preostalim pokušajima
     */
    VerificationResponseDto getVerificationStatus(Long sessionId);
}
