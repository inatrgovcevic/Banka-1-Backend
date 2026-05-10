package com.banka1.banking_service.card_service.dto.card_creation.internal;

import com.banka1.banking_service.card_service.domain.enums.CardBrand;

import java.math.BigDecimal;

/**
 * Internal command object for card creation.
 * This command aggregates all business inputs needed by {@code CardCreationService#createCard(...)},
 * so callers pass a single object instead of multiple scalar parameters.
 *
 * @param accountNumber linked account number
 * @param cardBrand requested card brand
 * @param cardLimit spending limit to persist on the created card
 * @param clientId owner client ID
 * @param authorizedPersonId optional authorized-person ID
 */
public record CreateCardCommand(
        String accountNumber,
        CardBrand cardBrand,
        BigDecimal cardLimit,
        Long clientId,
        Long authorizedPersonId
) {
}
