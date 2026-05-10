package com.banka1.banking_service.credit_service.rest_client;

import com.banka1.banking_service.credit_service.dto.response.VerificationStatusResponse;
import org.springframework.beans.factory.annotation.Qualifier;
import org.springframework.stereotype.Service;
import org.springframework.web.client.RestClient;

/**
 * REST Client for communicating with the Verification Service.
 * Provides methods for retrieving verification status information.
 */
@Service
public class VerificationService {

    private final RestClient restClient;

    /**
     * Constructs VerificationService with a qualified RestClient bean.
     *
     * @param restClient the RestClient bean configured for verification service communication
     */
    public VerificationService(@Qualifier("creditVerificationClient") RestClient restClient) {
        this.restClient = restClient;
    }

    /**
     * Retrieves the verification status for a given session.
     *
     * @param sessionId the verification session ID
     * @return VerificationStatusResponse containing the verification status
     */
    public VerificationStatusResponse getStatus(Long sessionId) {
        return restClient.get()
                .uri("/{sessionId}/status", sessionId)
                .retrieve()
                .body(VerificationStatusResponse.class);
    }
}
