package com.banka1.banking_service.transfer_service.client.impl;

import com.banka1.banking_service.transfer_service.client.VerificationClient;
import com.banka1.banking_service.transfer_service.dto.client.VerificationResponseDto;
import com.banka1.banking_service.transfer_service.exception.BusinessException;
import com.banka1.banking_service.transfer_service.exception.ErrorCode;
import lombok.RequiredArgsConstructor;
import org.springframework.context.annotation.Profile;
import org.springframework.stereotype.Component;
import org.springframework.web.client.HttpStatusCodeException;
import org.springframework.web.client.RestClient;

/**
 * Implementacija klijenta za verifikaciju putem POST zahteva.
 */
@Component
@Profile("!local")
@RequiredArgsConstructor
public class VerificationClientImpl implements VerificationClient {

    private final RestClient verificationRestClient;

    @Override
    public VerificationResponseDto getVerificationStatus(Long sessionId) {
        try {
            return verificationRestClient.get()
                    .uri("/{sessionId}/status", sessionId)
                    .retrieve()
                    .body(VerificationResponseDto.class);
        } catch (HttpStatusCodeException e) {
            throw new BusinessException(ErrorCode.INVALID_VERIFICATION, "Greška pri dohvatanju statusa verifikacije: " + e.getResponseBodyAsString());
        } catch (Exception e) {
            throw new BusinessException(ErrorCode.TRANSFER_NOT_FOUND, "Servis za verifikaciju nije dostupan.");
        }
    }
}
