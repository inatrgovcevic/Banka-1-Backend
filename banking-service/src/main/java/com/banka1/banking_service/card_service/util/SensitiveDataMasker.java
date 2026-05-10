package com.banka1.banking_service.card_service.util;

/**
 * Shared masking helpers for sensitive card-service identifiers.
 */
public final class SensitiveDataMasker {

    private SensitiveDataMasker() {
    }

    /**
     * Masks a card number so only the first 4 and last 4 digits remain visible.
     *
     * <p>Example: {@code 4111111111111111 -> 4111********1111}
     *
     * @param cardNumber raw card number
     * @return masked card number, or the original value when it is null or too short to mask safely
     */
    public static String maskCardNumber(String cardNumber) {
        if (cardNumber == null || cardNumber.length() <= 8) {
            return cardNumber;
        }
        return cardNumber.substring(0, 4)
                + "*".repeat(cardNumber.length() - 8)
                + cardNumber.substring(cardNumber.length() - 4);
    }

    /**
     * Masks an account number so only the last 4 characters remain visible.
     *
     * <p>Example: {@code 265000000000001234 -> **************1234}
     *
     * @param accountNumber raw account number
     * @return masked account number, or the original value when it is null or too short to mask safely
     */
    public static String maskAccountNumber(String accountNumber) {
        if (accountNumber == null || accountNumber.length() <= 4) {
            return accountNumber;
        }
        return "*".repeat(accountNumber.length() - 4) + accountNumber.substring(accountNumber.length() - 4);
    }
}
