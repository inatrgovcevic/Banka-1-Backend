package com.banka1.banking_service.card_service.rest_client;

import org.springframework.beans.factory.annotation.Qualifier;
import org.springframework.stereotype.Service;
import org.springframework.web.client.RestClient;

@Service
public class VerificationService {

    private final RestClient restClient;

    public VerificationService(@Qualifier("cardVerificationClient") RestClient restClient) {
        this.restClient = restClient;
    }

    public VerificationStatusResponse getStatus(Long verificationId) {
        return restClient.get()
                .uri("/{verificationId}/status", verificationId)        // sessionId
                .retrieve()
                .body(VerificationStatusResponse.class);
    }
}
