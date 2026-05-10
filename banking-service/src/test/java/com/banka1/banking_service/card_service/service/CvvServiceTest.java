package com.banka1.banking_service.card_service.service;

import com.banka1.banking_service.card_service.dto.card_creation.internal.GeneratedCvv;
import org.junit.jupiter.api.Test;
import org.springframework.security.crypto.argon2.Argon2PasswordEncoder;

import static org.junit.jupiter.api.Assertions.*;

class CvvServiceTest {

    private final CvvService cvvService = new CvvService(Argon2PasswordEncoder.defaultsForSpringSecurity_v5_8());

    @Test
    void generateCvvReturnsPlainThreeDigitValueAndMatchingHash() {
        GeneratedCvv generatedCvv = cvvService.generateCvv();

        assertTrue(generatedCvv.plainCvv().matches("\\d{3}"));
        assertTrue(cvvService.matches(generatedCvv.plainCvv(), generatedCvv.hashedCvv()));
        assertFalse(generatedCvv.plainCvv().equals(generatedCvv.hashedCvv()));
    }

    @Test
    void hashCvvRejectsValuesThatAreNotThreeDigits() {
        assertThrows(IllegalArgumentException.class, () -> cvvService.hashCvv("12"));
        assertThrows(IllegalArgumentException.class, () -> cvvService.hashCvv("12A"));
    }
}
