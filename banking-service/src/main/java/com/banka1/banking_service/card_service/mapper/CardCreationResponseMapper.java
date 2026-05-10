package com.banka1.banking_service.card_service.mapper;

import com.banka1.banking_service.card_service.domain.Card;
import com.banka1.banking_service.card_service.dto.card_creation.internal.CardCreationResult;
import com.banka1.banking_service.card_service.dto.card_creation.response.CardCreationResponseDto;
import org.springframework.stereotype.Component;

/**
 * Maps internal card-creation results to API response DTOs.
 */
@Component
public class CardCreationResponseMapper {

    public CardCreationResponseDto toDto(CardCreationResult result) {
        Card card = result.card();
        return new CardCreationResponseDto(
                card.getId(),
                card.getCardNumber(),
                result.plainCvv(),
                card.getExpirationDate(),
                card.getCardName()
        );
    }
}
