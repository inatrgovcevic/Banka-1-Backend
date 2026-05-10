package com.banka1.banking_service.transfer_service.repository;


import com.banka1.banking_service.transfer_service.domain.Transfer;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.data.jpa.test.autoconfigure.DataJpaTest;
import org.springframework.data.domain.Page;
import org.springframework.data.domain.PageRequest;

import java.math.BigDecimal;
import java.time.Instant;
import java.util.Optional;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertTrue;

@DataJpaTest // Podiže H2 bazu i skenira entitete
class TransferRepositoryTest {

    @Autowired
    private TransferRepository transferRepository;

    @BeforeEach
    void setUp() {
        transferRepository.deleteAll();

        Transfer t1 = Transfer.builder()
                .orderNumber("TRF-001")
                .clientId(1L)
                .fromAccountNumber("111")
                .toAccountNumber("222")
                .initialAmount(BigDecimal.TEN)
                .finalAmount(BigDecimal.TEN)
                .commission(BigDecimal.ZERO)
                .verificationSessionId("sess-1")
                .timestamp(Instant.now())
                .build();

        Transfer t2 = Transfer.builder()
                .orderNumber("TRF-002")
                .clientId(1L)
                .fromAccountNumber("222")
                .toAccountNumber("333")
                .initialAmount(BigDecimal.TEN)
                .finalAmount(BigDecimal.TEN)
                .commission(BigDecimal.ZERO)
                .verificationSessionId("sess-2")
                .timestamp(Instant.now())
                .build();

        transferRepository.save(t1);
        transferRepository.save(t2);
    }

    @Test
    void findByOrderNumber_ShouldReturnTransfer() {
        Optional<Transfer> found = transferRepository.findByOrderNumber("TRF-001");
        assertTrue(found.isPresent());
        assertEquals("111", found.get().getFromAccountNumber());
    }

    @Test
    void findByClientId_ShouldReturnPage() {
        Page<Transfer> page = transferRepository.findByClientId(1L, PageRequest.of(0, 10));
        assertEquals(2, page.getTotalElements());
    }

    @Test
    void existsByVerificationSessionId_ShouldReturnTrue() {
        boolean exists = transferRepository.existsByVerificationSessionId("sess-1");
        assertTrue(exists);
    }

    @Test
    void findByFromAccountNumberOrToAccountNumber_ShouldFindBothWays() {
        // Tražimo sve transfere gde se pojavljuje račun "222" (bio je primalac u t1, pošiljalac u t2)
        Page<Transfer> page = transferRepository.findByFromAccountNumberOrToAccountNumber("222", "222", PageRequest.of(0, 10));

        assertEquals(2, page.getTotalElements());
    }
}