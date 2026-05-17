package com.banka1.interbank.mock;

import static org.junit.jupiter.api.Assertions.assertTrue;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.post;

import com.banka1.interbank.protocol.dto.Asset;
import com.banka1.interbank.protocol.dto.CurrencyCode;
import com.banka1.interbank.protocol.dto.ForeignBankId;
import com.banka1.interbank.protocol.dto.IdempotenceKey;
import com.banka1.interbank.protocol.dto.InterbankMessagePayload;
import com.banka1.interbank.protocol.dto.InterbankTransactionPayload;
import com.banka1.interbank.protocol.dto.MessageType;
import com.banka1.interbank.protocol.dto.MonetaryAsset;
import com.banka1.interbank.protocol.dto.Posting;
import com.banka1.interbank.protocol.dto.TxAccount;
import com.banka1.interbank.client.TradingInternalClient;
import com.banka1.interbank.client.UserInternalClient;
import com.banka1.interbank.service.InterbankCoordinatorService;
import com.banka1.interbank.service.TransactionExecutorService;
import com.fasterxml.jackson.databind.ObjectMapper;
import java.math.BigDecimal;
import java.util.List;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.test.context.SpringBootTest;
import org.springframework.http.MediaType;
import org.springframework.test.context.ActiveProfiles;
import org.springframework.test.context.bean.override.mockito.MockitoBean;
import org.springframework.test.web.servlet.MockMvc;
import org.springframework.test.web.servlet.MvcResult;
import org.springframework.test.web.servlet.setup.MockMvcBuilders;
import org.springframework.web.context.WebApplicationContext;

/**
 * PR_32 Phase 9: verifikuje da {@link MockBanka2Controller} NIJE aktivan kad
 * {@code mock-partner} profil nije ukljucen.
 *
 * <p>Bez {@code mock-partner} profila, {@code @Profile("mock-partner")}
 * annotation na controller-u sprecava Spring da ga registruje kao bean. Posto
 * nema mapping handlera za {@code /_mock/banka2/**}, security-lib
 * {@code apiChain} filter chain odbija request — moze biti 401/403/404 u
 * zavisnosti od filter chain-a. Glavna stvar je da NE vraca 200/204 (sto bi
 * znacilo da mock zaista obradjuje request iako profile nije aktivan).
 */
@SpringBootTest
@ActiveProfiles("test")
class MockBanka2ControllerNotActiveTest {

    @Autowired private WebApplicationContext context;
    @Autowired private ObjectMapper mapper;

    // Stub TransactionExecutorService — vidi MockBanka2ControllerTest komentar.
    @MockitoBean private TransactionExecutorService executor;
    @MockitoBean private InterbankCoordinatorService coordinator;
    @MockitoBean private TradingInternalClient trading;
    @MockitoBean private UserInternalClient userClient;
    // PR_33 Phase A: InterbankOtcOutboundService zavisi od InterbankClient.
    @MockitoBean private com.banka1.interbank.service.InterbankClient interbankClient;

    private MockMvc mockMvc;

    @BeforeEach
    void setUp() {
        mockMvc = MockMvcBuilders
                .webAppContextSetup(context)
                .apply(org.springframework.security.test.web.servlet.setup.SecurityMockMvcConfigurers.springSecurity())
                .build();
    }

    @Test
    void mockControllerNotActiveWithoutProfile() throws Exception {
        Posting p1 = new Posting(
                new TxAccount.Account("111000000000000001"),
                new BigDecimal("-1000"),
                new Asset.Monas(new MonetaryAsset(CurrencyCode.USD)));
        Posting p2 = new Posting(
                new TxAccount.Account("222000000000000002"),
                new BigDecimal("1000"),
                new Asset.Monas(new MonetaryAsset(CurrencyCode.USD)));
        InterbankTransactionPayload tx = new InterbankTransactionPayload(
                List.of(p1, p2),
                new ForeignBankId(222, "no-profile-key"),
                "test", "1234", "289", "Test payment");
        InterbankMessagePayload msg = new InterbankMessagePayload(
                new IdempotenceKey(222, "no-profile-key"),
                MessageType.NEW_TX,
                mapper.valueToTree(tx));

        // Mock controller je @Profile("mock-partner") guard-ovan — bez profila
        // bean ne postoji i request ne moze biti obraden. Bilo koji od (401,
        // 403, 404) je validan signal "mock not active"; jedino 200/204 znaci
        // buggy setup gde bean curi van profila.
        MvcResult result = mockMvc.perform(post("/_mock/banka2/interbank")
                        .header("X-Api-Key", "test-inbound")
                        .contentType(MediaType.APPLICATION_JSON)
                        .content(mapper.writeValueAsString(msg)))
                .andReturn();

        int status = result.getResponse().getStatus();
        assertTrue(status == 401 || status == 403 || status == 404,
                "Expected 401/403/404 (mock not active), got " + status);
    }
}
