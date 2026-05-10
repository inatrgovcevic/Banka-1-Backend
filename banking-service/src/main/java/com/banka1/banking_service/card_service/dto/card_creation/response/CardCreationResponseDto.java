package com.banka1.banking_service.card_service.dto.card_creation.response;

import java.time.LocalDate;

/**
 * API response returned after a card has been created successfully.
 *
 * @param cardId stable card ID for subsequent card-management calls
 * @param cardNumber full card number
 * @param plainCvv one-time plain CVV
 * @param expirationDate expiration date of the new card
 * @param cardName display name of the product
 */
public record CardCreationResponseDto(
        Long cardId,
        String cardNumber,
        String plainCvv,
        LocalDate expirationDate,
        String cardName
) {
}
