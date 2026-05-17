package com.banka1.interbank.controller;

import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.Mockito.times;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.verifyNoInteractions;
import static org.mockito.Mockito.when;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.post;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.content;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.jsonPath;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.status;

import com.banka1.interbank.model.InterbankMessageEntity;
import com.banka1.interbank.model.enums.Direction;
import com.banka1.interbank.model.enums.MessageStatus;
import com.banka1.interbank.protocol.dto.Asset;
import com.banka1.interbank.protocol.dto.CommitTransactionBody;
import com.banka1.interbank.protocol.dto.CurrencyCode;
import com.banka1.interbank.protocol.dto.ForeignBankId;
import com.banka1.interbank.protocol.dto.IdempotenceKey;
import com.banka1.interbank.protocol.dto.InterbankMessagePayload;
import com.banka1.interbank.protocol.dto.InterbankTransactionPayload;
import com.banka1.interbank.protocol.dto.MessageType;
import com.banka1.interbank.protocol.dto.MonetaryAsset;
import com.banka1.interbank.protocol.dto.NoVoteReason;
import com.banka1.interbank.protocol.dto.Posting;
import com.banka1.interbank.protocol.dto.RollbackTransactionBody;
import com.banka1.interbank.protocol.dto.TransactionVote;
import com.banka1.interbank.protocol.dto.TxAccount;
import com.banka1.interbank.repository.InterbankMessageRepository;
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
 * PR_32 Phase 7 integration testovi za {@link InterbankInboundController} + {@link InboundDispatcher}.
 *
 * <p>Pokriva sledece slucajeve iz spec §6.1:
 * <ol>
 *   <li>{@code newTxYesHappyPath} — validni X-Api-Key + NEW_TX message, executor mock vraca
 *       YES → 200 OK + body sa {@code "vote": "YES"} + cache entry sacuvan.</li>
 *   <li>{@code newTxNoUnbalanced} — executor vraca NO sa UNBALANCED_TX → 200 OK + cache sacuvan.</li>
 *   <li>{@code commitTxIdempotent} — POST COMMIT_TX dva puta sa istim idempotency key →
 *       executor.commitLocal pozvan SAMO JEDNOM (drugi call vraca cached 204).</li>
 *   <li>{@code rollbackTxIdempotent} — slicno za ROLLBACK_TX.</li>
 *   <li>{@code retryReturnsCachedResponse} — POST NEW_TX dva puta, drugi vraca cached body bez
 *       ponovnog poziva executor-a.</li>
 *   <li>{@code missingApiKey401} — POST bez X-Api-Key → 401.</li>
 *   <li>{@code routingMismatch400} — POST sa idempotenceKey.routingNumber=999 ali X-Api-Key
 *       matchuje partner-a 222 → 400 Bad Request.</li>
 * </ol>
 *
 * <p><strong>Setup quirk:</strong> Spring Security {@code @MockitoBean} ne moze automatski da
 * inject-uje SecurityFilterChain — MockMvc se gradi rucno preko {@code WebApplicationContext}-a
 * sa {@code apply(springSecurity())} da bismo zadrzali InterbankAuthFilter chain. Test profil
 * koristi H2 in-memory bazu i Liquibase-off (per {@code application-test.properties}), pa se
 * {@code InterbankMessageRepository} ponasa kao prava JPA repo sa create-drop semom.
 */
@SpringBootTest
@ActiveProfiles("test")
class InterbankInboundControllerIntegrationTest {

    private static final String VALID_TOKEN = "test-inbound";
    private static final int PARTNER_ROUTING = 222;
    private static final int MY_ROUTING = 111;

    @Autowired private WebApplicationContext context;
    @Autowired private ObjectMapper mapper;
    @Autowired private InterbankMessageRepository messageRepo;

    @MockitoBean private TransactionExecutorService executor;
    @MockitoBean private InterbankCoordinatorService coordinator;
    @MockitoBean private TradingInternalClient trading;
    @MockitoBean private UserInternalClient userClient;
    // PR_33 Phase A: InterbankClient nije @Profile("test") za prod, ali
    // InterbankOtcOutboundService (NIJE @Profile("!test")) ga sad zavisi.
    // Mock ovde da context loaduje bez NoSuchBeanDefinitionException.
    @MockitoBean private com.banka1.interbank.service.InterbankClient interbankClient;

    private MockMvc mockMvc;

    @BeforeEach
    void setUp() {
        // Build MockMvc ručno tako da Spring Security filter chain (InterbankAuthFilter @Order(0))
        // ostane aktivan i validira X-Api-Key header per Phase 4 konfiguraciji.
        mockMvc = MockMvcBuilders
                .webAppContextSetup(context)
                .apply(org.springframework.security.test.web.servlet.setup.SecurityMockMvcConfigurers.springSecurity())
                .build();
        messageRepo.deleteAll();
    }

    // ===== Helpers ============================================================

    private InterbankMessagePayload newTxPayload(String key) throws Exception {
        InterbankTransactionPayload tx = sampleTx(key);
        return new InterbankMessagePayload(
                new IdempotenceKey(PARTNER_ROUTING, key),
                MessageType.NEW_TX,
                mapper.valueToTree(tx)
        );
    }

    private InterbankTransactionPayload sampleTx(String txId) {
        Posting p1 = new Posting(
                new TxAccount.Account("111000000000000001"),
                new BigDecimal("-1000"),
                new Asset.Monas(new MonetaryAsset(CurrencyCode.USD)));
        Posting p2 = new Posting(
                new TxAccount.Account("222000000000000002"),
                new BigDecimal("1000"),
                new Asset.Monas(new MonetaryAsset(CurrencyCode.USD)));
        return new InterbankTransactionPayload(
                List.of(p1, p2),
                new ForeignBankId(PARTNER_ROUTING, txId),
                "test", "1234", "289", "Test payment");
    }

    private InterbankMessagePayload commitPayload(String key, String txId) {
        return new InterbankMessagePayload(
                new IdempotenceKey(PARTNER_ROUTING, key),
                MessageType.COMMIT_TX,
                mapper.valueToTree(new CommitTransactionBody(new ForeignBankId(PARTNER_ROUTING, txId)))
        );
    }

    private InterbankMessagePayload rollbackPayload(String key, String txId) {
        return new InterbankMessagePayload(
                new IdempotenceKey(PARTNER_ROUTING, key),
                MessageType.ROLLBACK_TX,
                mapper.valueToTree(new RollbackTransactionBody(new ForeignBankId(PARTNER_ROUTING, txId)))
        );
    }

    // ===== Test 1: NEW_TX YES happy path =====================================

    @Test
    void newTxYesHappyPath() throws Exception {
        when(executor.prepareLocal(any())).thenReturn(TransactionVote.yes());
        InterbankMessagePayload msg = newTxPayload("key-yes-1");

        mockMvc.perform(post("/interbank")
                        .header("X-Api-Key", VALID_TOKEN)
                        .contentType(MediaType.APPLICATION_JSON)
                        .content(mapper.writeValueAsString(msg)))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.vote").value("YES"));

        verify(executor, times(1)).prepareLocal(any());
        // Cache entry persisted
        var cached = messageRepo.findByDirectionAndSenderRoutingNumberAndLocallyGeneratedKey(
                Direction.INBOUND, PARTNER_ROUTING, "key-yes-1");
        assertThat(cached).isPresent();
        assertThat(cached.get().getMessageType()).isEqualTo(MessageType.NEW_TX);
        assertThat(cached.get().getStatus()).isEqualTo(MessageStatus.INBOUND_PROCESSED);
        assertThat(cached.get().getHttpStatus()).isEqualTo(200);
        assertThat(cached.get().getResponseBody()).contains("YES");
    }

    // ===== Test 2: NEW_TX NO + UNBALANCED ====================================

    @Test
    void newTxNoUnbalanced() throws Exception {
        when(executor.prepareLocal(any())).thenReturn(
                TransactionVote.no(List.of(NoVoteReason.of(NoVoteReason.Reason.UNBALANCED_TX))));
        InterbankMessagePayload msg = newTxPayload("key-no-1");

        mockMvc.perform(post("/interbank")
                        .header("X-Api-Key", VALID_TOKEN)
                        .contentType(MediaType.APPLICATION_JSON)
                        .content(mapper.writeValueAsString(msg)))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.vote").value("NO"))
                .andExpect(jsonPath("$.reasons[0].reason").value("UNBALANCED_TX"));

        var cached = messageRepo.findByDirectionAndSenderRoutingNumberAndLocallyGeneratedKey(
                Direction.INBOUND, PARTNER_ROUTING, "key-no-1");
        assertThat(cached).isPresent();
        assertThat(cached.get().getResponseBody()).contains("UNBALANCED_TX");
    }

    // ===== Test 3: COMMIT_TX idempotent ======================================

    @Test
    void commitTxIdempotent() throws Exception {
        InterbankMessagePayload msg = commitPayload("commit-key-1", "TX-100");

        // Prvi POST
        mockMvc.perform(post("/interbank")
                        .header("X-Api-Key", VALID_TOKEN)
                        .contentType(MediaType.APPLICATION_JSON)
                        .content(mapper.writeValueAsString(msg)))
                .andExpect(status().isNoContent());

        // Drugi POST sa istim key — executor NE sme da bude pozvan ponovo (cache hit)
        mockMvc.perform(post("/interbank")
                        .header("X-Api-Key", VALID_TOKEN)
                        .contentType(MediaType.APPLICATION_JSON)
                        .content(mapper.writeValueAsString(msg)))
                .andExpect(status().isNoContent());

        verify(executor, times(1)).commitLocal(any());
    }

    // ===== Test 4: ROLLBACK_TX idempotent ====================================

    @Test
    void rollbackTxIdempotent() throws Exception {
        InterbankMessagePayload msg = rollbackPayload("rollback-key-1", "TX-200");

        mockMvc.perform(post("/interbank")
                        .header("X-Api-Key", VALID_TOKEN)
                        .contentType(MediaType.APPLICATION_JSON)
                        .content(mapper.writeValueAsString(msg)))
                .andExpect(status().isNoContent());

        mockMvc.perform(post("/interbank")
                        .header("X-Api-Key", VALID_TOKEN)
                        .contentType(MediaType.APPLICATION_JSON)
                        .content(mapper.writeValueAsString(msg)))
                .andExpect(status().isNoContent());

        verify(executor, times(1)).rollbackLocal(any());
    }

    // ===== Test 5: NEW_TX retry returns cached response ======================

    @Test
    void retryReturnsCachedResponse() throws Exception {
        when(executor.prepareLocal(any())).thenReturn(TransactionVote.yes());
        InterbankMessagePayload msg = newTxPayload("retry-key-1");

        // Prvi POST — obrada + cache
        mockMvc.perform(post("/interbank")
                        .header("X-Api-Key", VALID_TOKEN)
                        .contentType(MediaType.APPLICATION_JSON)
                        .content(mapper.writeValueAsString(msg)))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.vote").value("YES"));

        // Drugi POST sa istim key — vraca cached body bez novog poziva executor-a
        mockMvc.perform(post("/interbank")
                        .header("X-Api-Key", VALID_TOKEN)
                        .contentType(MediaType.APPLICATION_JSON)
                        .content(mapper.writeValueAsString(msg)))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.vote").value("YES"));

        verify(executor, times(1)).prepareLocal(any());
    }

    // ===== Test 6: Missing X-Api-Key → 401 ===================================

    @Test
    void missingApiKey401() throws Exception {
        InterbankMessagePayload msg = newTxPayload("no-auth-key");

        mockMvc.perform(post("/interbank")
                        .contentType(MediaType.APPLICATION_JSON)
                        .content(mapper.writeValueAsString(msg)))
                .andExpect(status().isUnauthorized());

        verifyNoInteractions(executor);
    }

    // ===== Test 7: Routing mismatch → 400 ====================================

    @Test
    void routingMismatch400() throws Exception {
        // idempotenceKey.routingNumber=999 ali X-Api-Key partner = 222 → mismatch
        InterbankTransactionPayload tx = sampleTx("mismatch-1");
        InterbankMessagePayload msg = new InterbankMessagePayload(
                new IdempotenceKey(999, "bad-routing-key"),
                MessageType.NEW_TX,
                mapper.valueToTree(tx)
        );

        mockMvc.perform(post("/interbank")
                        .header("X-Api-Key", VALID_TOKEN)
                        .contentType(MediaType.APPLICATION_JSON)
                        .content(mapper.writeValueAsString(msg)))
                .andExpect(status().isBadRequest())
                .andExpect(content().string(org.hamcrest.Matchers.containsString("mismatches")));

        verifyNoInteractions(executor);
    }
}
