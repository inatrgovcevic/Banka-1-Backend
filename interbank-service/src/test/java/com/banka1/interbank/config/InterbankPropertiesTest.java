package com.banka1.interbank.config;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.junit.jupiter.api.Assertions.assertTrue;

import java.util.Optional;
import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.context.properties.EnableConfigurationProperties;
import org.springframework.boot.test.context.SpringBootTest;
import org.springframework.context.annotation.Configuration;
import org.springframework.test.context.TestPropertySource;

/**
 * PR_32 Phase 4: testovi za {@link InterbankProperties} binding i lookup helper-e.
 *
 * <p>Koristimo izolovani {@link SpringBootTest} sa minimalnom test
 * konfiguracijom (samo @EnableConfigurationProperties) da binding ne zavisi od
 * pune {@code InterbankServiceApplication} context-load procedure (Liquibase,
 * RabbitMQ, etc.).
 */
@SpringBootTest(classes = InterbankPropertiesTest.TestConfig.class)
@TestPropertySource(properties = {
    "interbank.my-routing-number=111",
    "interbank.my-bank-display-name=Banka 1",
    "interbank.partners[0].routing-number=222",
    "interbank.partners[0].display-name=Banka 2",
    "interbank.partners[0].base-url=http://banka2.local/",
    "interbank.partners[0].inbound-token=tok-in-222",
    "interbank.partners[0].outbound-token=tok-out-222",
    "interbank.partners[1].routing-number=333",
    "interbank.partners[1].display-name=Banka 3",
    "interbank.partners[1].base-url=http://banka3.local/",
    "interbank.partners[1].inbound-token=tok-in-333",
    "interbank.partners[1].outbound-token=tok-out-333"
})
class InterbankPropertiesTest {

    @Configuration
    @EnableConfigurationProperties(InterbankProperties.class)
    static class TestConfig {
    }

    @Autowired
    private InterbankProperties properties;

    @Test
    void loadsConfig() {
        assertEquals(111, properties.getMyRoutingNumber());
        assertEquals("Banka 1", properties.getMyBankDisplayName());
        assertEquals(2, properties.getPartners().size(),
                "partners lista mora imati oba unosa iz property-ja");

        InterbankProperties.Partner first = properties.getPartners().get(0);
        assertEquals(222, first.getRoutingNumber());
        assertEquals("Banka 2", first.getDisplayName());
        assertEquals("http://banka2.local/", first.getBaseUrl());
        assertEquals("tok-in-222", first.getInboundToken());
        assertEquals("tok-out-222", first.getOutboundToken());
    }

    @Test
    void findByInboundToken() {
        Optional<InterbankProperties.Partner> match =
                properties.findByInboundToken("tok-in-333");
        assertTrue(match.isPresent(), "lookup po validnom tokenu mora vratiti partnera");
        assertEquals(333, match.get().getRoutingNumber());

        assertFalse(properties.findByInboundToken("nepostojeci-token").isPresent(),
                "lookup po nepostojecem tokenu mora vratiti empty Optional");
        assertFalse(properties.findByInboundToken(null).isPresent(),
                "null token mora vratiti empty Optional bez NPE");
        assertFalse(properties.findByInboundToken("").isPresent(),
                "prazan token mora vratiti empty Optional");
    }

    @Test
    void partnerOrThrow() {
        InterbankProperties.Partner p = properties.partnerOrThrow(222);
        assertEquals("Banka 2", p.getDisplayName());

        IllegalArgumentException ex = assertThrows(
                IllegalArgumentException.class,
                () -> properties.partnerOrThrow(999),
                "nepostojeci routing number mora baciti exception"
        );
        assertTrue(ex.getMessage().contains("999"),
                "exception poruka mora sadrzati trazeni routing number, bila: " + ex.getMessage());
    }
}
