package com.banka1.banking_service.card_service.dto.card_management.response;

import com.banka1.banking_service.card_service.domain.Card;
import com.banka1.banking_service.card_service.util.SensitiveDataMasker;
import lombok.Getter;

/**
 * Compact card representation used in list responses.
 * The card ID is included so callers can fetch full details later
 * without using the sensitive card number as a lookup key.
 * The card number is masked to protect sensitive data - only the first four
 * and last four digits are visible, with asterisks replacing the middle digits.
 *
 * Example:
 * a card number {@code 5798123456785571} is returned as {@code 5798********5571}.
 *
 * The CVV is never included in any list or detail response.
 */
@Getter
public class CardSummaryDTO {

    /**
     * Stable card identifier safe to use in follow-up API calls.
     */
    private final Long id;

    /**
     * Masked card number safe for display in lists.
     * Format: first 4 digits + 8 asterisks + last 4 digits.
     */
    private final String maskedCardNumber;

    private final String accountNumber;

    public CardSummaryDTO(Card card) {
        this.id = card.getId();
        this.maskedCardNumber = SensitiveDataMasker.maskCardNumber(card.getCardNumber());
        this.accountNumber = card.getAccountNumber();
    }
}
