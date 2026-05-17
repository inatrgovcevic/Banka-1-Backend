package com.banka1.interbank.mock;

import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.get;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.post;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.jsonPath;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.status;

import com.banka1.interbank.protocol.dto.CommitTransactionBody;
import com.banka1.interbank.protocol.dto.ForeignBankId;
import com.banka1.interbank.protocol.dto.IdempotenceKey;
import com.banka1.interbank.protocol.dto.InterbankMessagePayload;
import com.banka1.interbank.protocol.dto.InterbankTransactionPayload;
import com.banka1.interbank.protocol.dto.MessageType;
import com.banka1.interbank.protocol.dto.Asset;
import com.banka1.interbank.protocol.dto.CurrencyCode;
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
import org.springframework.test.web.servlet.setup.MockMvcBuilders;
import org.springframework.web.context.WebApplicationContext;

/**
 * PR_32 Phase 9 testovi za {@link MockBanka2Controller}.
 *
 * <p>Pokriva 5 scenarija mocked Banka 2 endpoint-a:
 * <ol>
 *   <li>{@code mockInterbankNewTxYes} — NEW_TX bez query parametara → YES vote.</li>
 *   <li>{@code mockInterbankNewTxNo} — NEW_TX sa {@code ?vote=NO&reason=INSUFFICIENT_ASSET}
 *       → NO vote sa odgovarajucim razlogom.</li>
 *   <li>{@code mockInterbankCommitReturns204} — COMMIT_TX → 204 No Content.</li>
 *   <li>{@code mockPublicStockReturnsCannedList} — GET /public-stock → lista sa
 *       2 ticker-a (TSLA, MSFT).</li>
 *   <li>{@code mockInterbankMissingApiKey401} — bez X-Api-Key → 401.</li>
 * </ol>
 *
 * <p><strong>Profile setup:</strong> {@code mock-partner} profile mora biti
 * aktivan da bi @{@link org.springframework.context.annotation.Profile}-guard-ovani
 * {@code MockBanka2Controller} bio bean. {@code application-mock-partner.properties}
 * dodaje {@code /_mock/**} u permit-all listu — bez ovoga security-lib
 * {@code apiChain} odbija nepoznate rute sa 401.
 */
@SpringBootTest
@ActiveProfiles({"test", "mock-partner"})
class MockBanka2ControllerTest {

    private static final String VALID_TOKEN = "test-inbound";
    private static final int PARTNER_ROUTING = 222;

    @Autowired private WebApplicationContext context;
    @Autowired private ObjectMapper mapper;

    // TransactionExecutorService je @Profile("!test") — u test profilu ne postoji,
    // pa InboundDispatcher (kome je dependency) ne moze da se kreira. @MockitoBean
    // registruje Mockito stub kao bean da bi ApplicationContext mogao da se digne.
    // Sami mock-tests ne pozivaju /interbank inbound rutu pa stub nikad nije korisen.
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

    private InterbankMessagePayload newTxPayload(String key) {
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
                new ForeignBankId(PARTNER_ROUTING, key),
                "test", "1234", "289", "Test payment");
        return new InterbankMessagePayload(
                new IdempotenceKey(PARTNER_ROUTING, key),
                MessageType.NEW_TX,
                mapper.valueToTree(tx));
    }

    private InterbankMessagePayload commitPayload(String key, String txId) {
        return new InterbankMessagePayload(
                new IdempotenceKey(PARTNER_ROUTING, key),
                MessageType.COMMIT_TX,
                mapper.valueToTree(new CommitTransactionBody(
                        new ForeignBankId(PARTNER_ROUTING, txId))));
    }

    // ===== Test 1: NEW_TX → YES (default) ====================================

    @Test
    void mockInterbankNewTxYes() throws Exception {
        InterbankMessagePayload msg = newTxPayload("mock-yes-1");

        mockMvc.perform(post("/_mock/banka2/interbank")
                        .header("X-Api-Key", VALID_TOKEN)
                        .contentType(MediaType.APPLICATION_JSON)
                        .content(mapper.writeValueAsString(msg)))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.vote").value("YES"));
    }

    // ===== Test 2: NEW_TX + ?vote=NO&reason=INSUFFICIENT_ASSET ===============

    @Test
    void mockInterbankNewTxNo() throws Exception {
        InterbankMessagePayload msg = newTxPayload("mock-no-1");

        mockMvc.perform(post("/_mock/banka2/interbank")
                        .header("X-Api-Key", VALID_TOKEN)
                        .param("vote", "NO")
                        .param("reason", "INSUFFICIENT_ASSET")
                        .contentType(MediaType.APPLICATION_JSON)
                        .content(mapper.writeValueAsString(msg)))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.vote").value("NO"))
                .andExpect(jsonPath("$.reasons[0].reason").value("INSUFFICIENT_ASSET"));
    }

    // ===== Test 3: COMMIT_TX → 204 ===========================================

    @Test
    void mockInterbankCommitReturns204() throws Exception {
        InterbankMessagePayload msg = commitPayload("mock-commit-1", "TX-mock-100");

        mockMvc.perform(post("/_mock/banka2/interbank")
                        .header("X-Api-Key", VALID_TOKEN)
                        .contentType(MediaType.APPLICATION_JSON)
                        .content(mapper.writeValueAsString(msg)))
                .andExpect(status().isNoContent());
    }

    // ===== Test 4: GET /public-stock → canned list ===========================

    @Test
    void mockPublicStockReturnsCannedList() throws Exception {
        mockMvc.perform(get("/_mock/banka2/public-stock")
                        .header("X-Api-Key", VALID_TOKEN))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.length()").value(2))
                .andExpect(jsonPath("$[0].stock.ticker").value("TSLA"))
                .andExpect(jsonPath("$[1].stock.ticker").value("MSFT"))
                .andExpect(jsonPath("$[0].sellers[0].seller.routingNumber").value(PARTNER_ROUTING))
                .andExpect(jsonPath("$[0].sellers[0].amount").value(25));
    }

    // ===== Test 5: Missing X-Api-Key → 401 ===================================

    @Test
    void mockInterbankMissingApiKey401() throws Exception {
        InterbankMessagePayload msg = newTxPayload("mock-no-auth-1");

        mockMvc.perform(post("/_mock/banka2/interbank")
                        .contentType(MediaType.APPLICATION_JSON)
                        .content(mapper.writeValueAsString(msg)))
                .andExpect(status().isUnauthorized());
    }
}
