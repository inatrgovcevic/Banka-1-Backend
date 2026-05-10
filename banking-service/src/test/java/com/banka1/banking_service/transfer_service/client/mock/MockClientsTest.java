package com.banka1.banking_service.transfer_service.client.mock;

import com.banka1.banking_service.account_service.dto.request.PaymentDto;

import com.banka1.banking_service.transfer_service.dto.client.*;
import com.banka1.banking_service.transfer_service.dto.responses.*;
import org.junit.jupiter.api.Test;
import static org.junit.jupiter.api.Assertions.*;

import java.math.BigDecimal;

import static org.junit.jupiter.api.Assertions.assertNotNull;

class MockClientsTest {

    @Test
    void testMockAccountClient() {
        MockAccountClient client = new MockAccountClient();
        AccountDto details = client.getAccountDetails("123");
        assertNotNull(details);
        UpdatedBalanceResponseDto transfer = client.executeTransfer(new PaymentDto("1", "2", BigDecimal.TEN, BigDecimal.TEN, BigDecimal.ZERO, 1L));
        assertNotNull(transfer);
    }

    @Test
    void testMockClientClient() {
        MockClientClient client = new MockClientClient();
        assertNotNull(client.getClientDetails(1L));
    }

    @Test
    void testMockExchangeClient() {
        MockExchangeClient client = new MockExchangeClient();
        ExchangeResponseDto diff = client.calculateExchange("RSD", "EUR", new BigDecimal("100"));
        assertEquals(new BigDecimal("104.00"), diff.toAmount());
    }

    @Test
    void testMockVerificationClient() {
        MockVerificationClient client = new MockVerificationClient();

        VerificationResponseDto response = client.getVerificationStatus(123L);
        assertTrue(response.isVerified());
        assertEquals("VERIFIED", response.status());
    }
}