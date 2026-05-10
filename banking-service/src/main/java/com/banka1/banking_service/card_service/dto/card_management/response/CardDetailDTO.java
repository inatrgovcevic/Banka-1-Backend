package com.banka1.banking_service.card_service.dto.card_management.response;

import com.banka1.banking_service.card_service.domain.Card;
import com.banka1.banking_service.card_service.domain.enums.CardStatus;
import com.banka1.banking_service.card_service.domain.enums.CardType;
import com.banka1.banking_service.card_service.dto.card_creation.internal.CardCreationResult;
import lombok.Getter;

import java.math.BigDecimal;
import java.time.LocalDate;

/**
 * Full card details returned for a single-card lookup.
 * The card ID is included so clients can correlate this payload with list responses.
 * The full card number is exposed here, so this response must only be returned
 * to the card owner or an authorized employee - never in bulk list responses.
 *
 * The CVV hash is intentionally excluded from all API responses.
 * The plain CVV is available only at card creation time via {@link CardCreationResult}.
 */
@Getter
public class CardDetailDTO {

    private final Long id;
    private final String cardNumber;
    private final CardType cardType;
    private final String cardName;
    private final LocalDate creationDate;
    private final LocalDate expirationDate;
    private final String accountNumber;
    private final BigDecimal cardLimit;
    private final CardStatus status;

    public CardDetailDTO(Card card) {
        this.id = card.getId();
        this.cardNumber = card.getCardNumber();
        this.cardType = card.getCardType();
        this.cardName = card.getCardName();
        this.creationDate = card.getCreationDate();
        this.expirationDate = card.getExpirationDate();
        this.accountNumber = card.getAccountNumber();
        this.cardLimit = card.getCardLimit();
        this.status = card.getStatus();
    }
}
