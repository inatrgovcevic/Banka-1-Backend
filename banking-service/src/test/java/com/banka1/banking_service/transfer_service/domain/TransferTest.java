package com.banka1.banking_service.transfer_service.domain;

import org.junit.jupiter.api.Test;

import static org.junit.jupiter.api.Assertions.assertNotNull;
import static org.junit.jupiter.api.Assertions.assertNull;

class TransferEntityTest {

    @Test
    void onCreate_ShouldSetTimestamps() {
        Transfer transfer = new Transfer();
        assertNull(transfer.getCreatedAt());

        transfer.onCreate(); // Simuliramo JPA callback

        assertNotNull(transfer.getCreatedAt());
        assertNotNull(transfer.getUpdatedAt());
        assertNotNull(transfer.getTimestamp());
    }
}