package com.banka1.interbank.service;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

import com.banka1.interbank.config.InterbankProperties;
import java.util.List;
import java.util.Set;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;

/**
 * PR_32 Phase 8 unit testovi za {@link BankRoutingService}.
 */
class BankRoutingServiceTest {

    private InterbankProperties props;
    private BankRoutingService routing;

    @BeforeEach
    void setUp() {
        props = new InterbankProperties();
        props.setMyRoutingNumber(111);
        props.setMyBankDisplayName("Banka 1");

        InterbankProperties.Partner banka2 = new InterbankProperties.Partner();
        banka2.setRoutingNumber(222);
        banka2.setDisplayName("Banka 2");
        banka2.setBaseUrl("http://banka2.local/");
        banka2.setInboundToken("in-222");
        banka2.setOutboundToken("out-222");

        InterbankProperties.Partner banka3 = new InterbankProperties.Partner();
        banka3.setRoutingNumber(333);
        banka3.setDisplayName("Banka 3");
        banka3.setBaseUrl("http://banka3.local/");
        banka3.setInboundToken("in-333");
        banka3.setOutboundToken("out-333");

        props.setPartners(List.of(banka2, banka3));
        routing = new BankRoutingService(props);
    }

    @Test
    void resolvePartnerByRouting_returnsPartnerForKnownRouting() {
        InterbankProperties.Partner p = routing.resolvePartnerByRouting(222);
        assertThat(p.getDisplayName()).isEqualTo("Banka 2");
        assertThat(p.getBaseUrl()).isEqualTo("http://banka2.local/");
    }

    @Test
    void resolvePartnerByRouting_throwsForUnknownRouting() {
        assertThatThrownBy(() -> routing.resolvePartnerByRouting(999))
                .isInstanceOf(IllegalArgumentException.class)
                .hasMessageContaining("999");
    }

    @Test
    void distinctPartnerRoutings_filtersOutMyRoutingAndDeduplicates() {
        Set<Integer> result = routing.distinctPartnerRoutings(
                List.of(111, 222, 222, 333, 111, 222));
        assertThat(result).containsExactlyInAnyOrder(222, 333);
    }

    @Test
    void isMine_returnsTrueForMyRoutingFalseForOthers() {
        assertThat(routing.isMine(111)).isTrue();
        assertThat(routing.isMine(222)).isFalse();
        assertThat(routing.isMine(999)).isFalse();
    }
}
