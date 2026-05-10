package com.banka1.banking_service.card_service.service;

import com.banka1.banking_service.card_service.dto.card_creation.internal.CardCreationResult;
import com.banka1.banking_service.card_service.dto.card_creation.internal.CreateCardCommand;

/**
 * Application-service contract responsible for creating new debit cards.
 * The implementation is expected to validate input, generate a unique brand-compliant card number,
 * generate and hash a CVV, set derived defaults such as card type and expiration date,
 * and persist the resulting entity.
 */
public interface CardCreationService {

    /**
     * Creates and persists a debit card with generated card number and CVV.
     * The incoming {@link CreateCardCommand} carries all creation inputs:
     * linked account number, selected brand, card limit, owner client ID,
     * and optional authorized-person ID for business-account cards.
     *
     * @param command internal create-card command with all required creation data
     * @return created card together with the one-time plain CVV
     */
    CardCreationResult createCard(CreateCardCommand command);
}
