package com.banka1.banking_service.card_service.dto.card_management.response;

import com.banka1.banking_service.card_service.domain.Card;
import com.banka1.banking_service.card_service.util.SensitiveDataMasker;
import lombok.Getter;

/**
 * Internal service-to-service card summary returned to trusted callers with the {@code SERVICE} role.
 * Field names intentionally mirror the consumer DTO on the account-service side so Jackson can
 * deserialize the payload directly without an intermediate mapping step.
 *
 * The card number is masked for the same reason as {@link CardSummaryDTO} — downstream callers
 * only need the last four digits for display.
 */
@Getter
public class CardInternalSummaryDTO {

    private final Long id;
    private final String cardNumber;
    private final String cardType;
    private final String status;
    private final String expiryDate;
    private final String accountNumber;

    public CardInternalSummaryDTO(Card card) {
        this.id = card.getId();
        this.cardNumber = SensitiveDataMasker.maskCardNumber(card.getCardNumber());
        this.cardType = card.getCardName();
        this.status = card.getStatus() == null ? null : card.getStatus().name();
        this.expiryDate = card.getExpirationDate() == null ? null : card.getExpirationDate().toString();
        this.accountNumber = card.getAccountNumber();
    }
}