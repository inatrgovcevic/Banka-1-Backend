package com.banka1.banking_service.card_service.util;

import org.junit.jupiter.api.Test;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertNull;

class SensitiveDataMaskerTest {

    @Test
    void maskCardNumberKeepsFirstAndLastFourDigitsVisible() {
        assertEquals("4111********1111", SensitiveDataMasker.maskCardNumber("4111111111111111"));
    }

    @Test
    void maskCardNumberReturnsOriginalValueWhenTooShortToMask() {
        assertEquals("12345678", SensitiveDataMasker.maskCardNumber("12345678"));
        assertNull(SensitiveDataMasker.maskCardNumber(null));
    }

    @Test
    void maskAccountNumberKeepsOnlyLastFourCharactersVisible() {
        assertEquals("**************1234", SensitiveDataMasker.maskAccountNumber("265000000000001234"));
    }

    @Test
    void maskAccountNumberReturnsOriginalValueWhenTooShortToMask() {
        assertEquals("1234", SensitiveDataMasker.maskAccountNumber("1234"));
        assertNull(SensitiveDataMasker.maskAccountNumber(null));
    }
}
