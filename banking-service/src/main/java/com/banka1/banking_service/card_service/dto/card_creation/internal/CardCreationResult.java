package com.banka1.banking_service.card_service.dto.card_creation.internal;

import com.banka1.banking_service.card_service.domain.Card;

/**
 * Internal result returned after a new card is created.
 *
 * This intentionally contains both the persisted {@link Card} entity
 * and the ONE-TIME plain CVV that can be shown to the caller immediately after creation.
 *
 * Example:
 * the caller may receive {@code plainCvv = "123"} once,
 * while {@code card.getCvv()} contains only the hash.
 *
 * @param card persisted card entity
 * @param plainCvv one-time plain CVV value
 */
public record CardCreationResult(
        Card card,
        String plainCvv
) {
}
