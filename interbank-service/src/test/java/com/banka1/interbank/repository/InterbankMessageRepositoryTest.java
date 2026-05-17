package com.banka1.interbank.repository;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.junit.jupiter.api.Assertions.assertTrue;

import com.banka1.interbank.model.InterbankMessageEntity;
import com.banka1.interbank.model.enums.Direction;
import com.banka1.interbank.model.enums.MessageStatus;
import com.banka1.interbank.protocol.dto.MessageType;
import java.util.Optional;
import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.data.jpa.test.autoconfigure.DataJpaTest;
import org.springframework.boot.jdbc.test.autoconfigure.AutoConfigureTestDatabase;
import org.springframework.dao.DataIntegrityViolationException;
import org.springframework.test.context.ActiveProfiles;

/**
 * PR_32 Phase 3: Spring Data slice test za {@link InterbankMessageRepository}.
 *
 * <p>Koristi H2 in-memory bazu (configured u {@code application-test.properties})
 * sa {@code spring.jpa.hibernate.ddl-auto=create-drop} — Liquibase je iskljucen
 * jer SQL changeset koristi PostgreSQL-specificne tipove (JSONB). Testcontainers
 * varijanta dolazi u Phase 6 integration testovima.
 *
 * <p>{@code @AutoConfigureTestDatabase(replace = NONE)} sprecava Spring da
 * automatski overrid-uje datasource — koristimo eksplicitnu H2 konfiguraciju
 * iz {@code application-test.properties}.
 */
@DataJpaTest
@ActiveProfiles("test")
@AutoConfigureTestDatabase(replace = AutoConfigureTestDatabase.Replace.NONE)
class InterbankMessageRepositoryTest {

    @Autowired
    private InterbankMessageRepository repo;

    @Test
    void uniqueConstraintEnforced() {
        InterbankMessageEntity m1 = InterbankMessageEntity.builder()
            .direction(Direction.INBOUND)
            .senderRoutingNumber(111)
            .locallyGeneratedKey("key-1")
            .messageType(MessageType.NEW_TX)
            .status(MessageStatus.INBOUND_PROCESSED)
            .requestBody("{}")
            .retryCount(0)
            .build();
        repo.saveAndFlush(m1);

        InterbankMessageEntity dupe = InterbankMessageEntity.builder()
            .direction(Direction.INBOUND)
            .senderRoutingNumber(111)
            .locallyGeneratedKey("key-1")
            .messageType(MessageType.NEW_TX)
            .status(MessageStatus.INBOUND_PROCESSED)
            .requestBody("{}")
            .retryCount(0)
            .build();

        assertThrows(
            DataIntegrityViolationException.class,
            () -> repo.saveAndFlush(dupe),
            "Unique (direction, sender_routing_number, locally_generated_key) "
                + "mora da odbije drugi insert sa istim trojkom."
        );
    }

    @Test
    void findByLookupKey() {
        repo.saveAndFlush(InterbankMessageEntity.builder()
            .direction(Direction.INBOUND)
            .senderRoutingNumber(222)
            .locallyGeneratedKey("found-me")
            .messageType(MessageType.NEW_TX)
            .status(MessageStatus.INBOUND_PROCESSED)
            .requestBody("{}")
            .retryCount(0)
            .build());

        Optional<InterbankMessageEntity> found =
            repo.findByDirectionAndSenderRoutingNumberAndLocallyGeneratedKey(
                Direction.INBOUND, 222, "found-me");

        assertTrue(found.isPresent(), "lookup po (INBOUND, 222, found-me) mora vratiti rezultat");
        assertEquals(MessageType.NEW_TX, found.get().getMessageType());
    }
}
