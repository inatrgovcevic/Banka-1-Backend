package com.banka1.banking_service.card_service.dto.card_creation.response;

import com.banka1.banking_service.card_service.dto.card_creation.response.CardCreationResponseDto;

/**
 * Response for client-initiated card request flows.
 *
 * @param status current flow status
 * @param message human-readable result message
 * @param verificationRequestId legacy field, unused in the current externally verified flow
 * @param createdCard created card payload for completed requests, including the stable card ID
 */
public record CardRequestResponseDto(
        String status,
        String message,
        Long verificationRequestId,
        CardCreationResponseDto createdCard
) {
}
