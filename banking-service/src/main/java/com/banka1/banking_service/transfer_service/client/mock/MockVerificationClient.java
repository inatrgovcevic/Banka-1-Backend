package com.banka1.banking_service.transfer_service.client.mock;

import com.banka1.banking_service.transfer_service.client.VerificationClient;
import com.banka1.banking_service.transfer_service.dto.client.VerificationResponseDto;
import lombok.extern.slf4j.Slf4j;
import org.springframework.context.annotation.Profile;
import org.springframework.stereotype.Component;

@Slf4j
@Component
@Profile("local")
public class MockVerificationClient implements VerificationClient {
    @Override
    public VerificationResponseDto getVerificationStatus(Long sessionId) {
        log.info("MOCK: Fetching verification status for session {}", sessionId);
        // Uvek vraćamo VERIFIED na lokalu da bi testiranje transfera prolazilo
        return new VerificationResponseDto(sessionId, "VERIFIED");
    }
}
