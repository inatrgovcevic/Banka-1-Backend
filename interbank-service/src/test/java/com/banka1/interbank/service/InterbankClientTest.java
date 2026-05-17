package com.banka1.interbank.service;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.Mockito.times;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;
import static org.springframework.test.web.client.match.MockRestRequestMatchers.content;
import static org.springframework.test.web.client.match.MockRestRequestMatchers.header;
import static org.springframework.test.web.client.match.MockRestRequestMatchers.jsonPath;
import static org.springframework.test.web.client.match.MockRestRequestMatchers.method;
import static org.springframework.test.web.client.match.MockRestRequestMatchers.requestTo;
import static org.springframework.test.web.client.response.MockRestResponseCreators.withNoContent;
import static org.springframework.test.web.client.response.MockRestResponseCreators.withServerError;
import static org.springframework.test.web.client.response.MockRestResponseCreators.withSuccess;

import com.banka1.interbank.config.InterbankProperties;
import com.banka1.interbank.model.InterbankMessageEntity;
import com.banka1.interbank.model.enums.Direction;
import com.banka1.interbank.model.enums.MessageStatus;
import com.banka1.interbank.protocol.dto.Asset;
import com.banka1.interbank.protocol.dto.CurrencyCode;
import com.banka1.interbank.protocol.dto.ForeignBankId;
import com.banka1.interbank.protocol.dto.InterbankMessagePayload;
import com.banka1.interbank.protocol.dto.InterbankTransactionPayload;
import com.banka1.interbank.protocol.dto.MessageType;
import com.banka1.interbank.protocol.dto.MonetaryAsset;
import com.banka1.interbank.protocol.dto.Posting;
import com.banka1.interbank.protocol.dto.TransactionVote;
import com.banka1.interbank.protocol.dto.TxAccount;
import com.banka1.interbank.repository.InterbankMessageRepository;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.datatype.jsr310.JavaTimeModule;
import java.math.BigDecimal;
import java.time.Instant;
import java.util.List;
import org.hamcrest.Matchers;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.ArgumentCaptor;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;
import org.springframework.http.HttpMethod;
import org.springframework.http.MediaType;
import org.springframework.http.converter.json.MappingJackson2HttpMessageConverter;
import org.springframework.test.web.client.MockRestServiceServer;
import org.springframework.web.client.RestClient;

/**
 * PR_32 Phase 8 unit testovi za {@link InterbankClient}.
 *
 * <p>Koristimo {@link MockRestServiceServer} bound za RestClient.Builder pattern (kao
 * Phase 5 BankingCoreInternalClientTest) da bismo izverifikovali HTTP request shape
 * bez Spring konteksta. {@link InterbankMessageRepository} je Mockito mock.
 */
@ExtendWith(MockitoExtension.class)
class InterbankClientTest {

    private MockRestServiceServer mockServer;
    private InterbankClient client;
    private InterbankProperties props;
    private ObjectMapper mapper;

    @Mock
    private InterbankMessageRepository msgRepo;

    @BeforeEach
    void setUp() {
        mapper = new ObjectMapper();
        mapper.registerModule(new JavaTimeModule());

        // RestClient builder mora da ima Jackson 2 message converter inace
        // InterbankMessagePayload.message (Jackson 2 JsonNode) se serijalizuje kao string
        // umesto kao nested object — vidi RestClientConfig#interbankOutboundRestClient.
        MappingJackson2HttpMessageConverter jackson2 = new MappingJackson2HttpMessageConverter(mapper);
        jackson2.setSupportedMediaTypes(List.of(MediaType.APPLICATION_JSON));

        RestClient.Builder builder = RestClient.builder()
                .messageConverters(converters -> converters.add(0, jackson2));
        mockServer = MockRestServiceServer.bindTo(builder).build();
        RestClient restClient = builder.build();

        props = new InterbankProperties();
        props.setMyRoutingNumber(111);
        props.setMyBankDisplayName("Banka 1");

        InterbankProperties.Partner banka2 = new InterbankProperties.Partner();
        banka2.setRoutingNumber(222);
        banka2.setDisplayName("Banka 2");
        banka2.setBaseUrl("http://banka2.local/");
        banka2.setInboundToken("in-222");
        banka2.setOutboundToken("out-222");
        props.setPartners(List.of(banka2));

        // Echo save: vrati prosledjeni entity bez izmena (simulira JPA persist bez ID-generation
        // jer testovi ne porede ID).
        when(msgRepo.save(any(InterbankMessageEntity.class)))
                .thenAnswer(inv -> inv.getArgument(0));

        client = new InterbankClient(restClient, props, msgRepo, mapper);
    }

    private InterbankTransactionPayload sampleTx() {
        Posting p = new Posting(
                new TxAccount.Account("222000001234567890"),
                new BigDecimal("100.50"),
                new Asset.Monas(new MonetaryAsset(CurrencyCode.EUR)));
        return new InterbankTransactionPayload(
                List.of(p),
                new ForeignBankId(111, "tx-local-1"),
                "test message",
                "97",
                "289",
                "Test placanje");
    }

    @Test
    void sendNewTx_happyPath_returnsYesVoteAndPersistsLog() {
        mockServer.expect(requestTo("http://banka2.local/interbank"))
                .andExpect(method(HttpMethod.POST))
                .andExpect(header("X-Api-Key", "out-222"))
                .andExpect(jsonPath("$.idempotenceKey.routingNumber").value(111))
                .andExpect(jsonPath("$.messageType").value("NEW_TX"))
                .andExpect(jsonPath("$.message.transactionId.id").value("tx-local-1"))
                .andRespond(withSuccess("{\"vote\":\"YES\"}", MediaType.APPLICATION_JSON));

        TransactionVote vote = client.sendNewTx(222, sampleTx());

        assertThat(vote.isYes()).isTrue();
        mockServer.verify();

        // Persist twice: once PENDING_SEND (saves.get(0)), once SENT (saves.get(1)).
        // NB: oba save-a su pozvana sa ISTOM instancom entity-ja (markSent mutira in-place),
        // pa ArgumentCaptor sadrzi reference na isti objekat — final state se vidi i u
        // get(0). Zato proveravamo stable invariants (direction, sender, type) na get(0)
        // i final state (status SENT, httpStatus, body) na get(1).
        ArgumentCaptor<InterbankMessageEntity> cap = ArgumentCaptor.forClass(InterbankMessageEntity.class);
        verify(msgRepo, times(2)).save(cap.capture());
        List<InterbankMessageEntity> saves = cap.getAllValues();
        assertThat(saves.get(0).getDirection()).isEqualTo(Direction.OUTBOUND);
        assertThat(saves.get(0).getSenderRoutingNumber()).isEqualTo(222);
        assertThat(saves.get(0).getMessageType()).isEqualTo(MessageType.NEW_TX);
        assertThat(saves.get(1).getStatus()).isEqualTo(MessageStatus.SENT);
        assertThat(saves.get(1).getHttpStatus()).isEqualTo(200);
        assertThat(saves.get(1).getResponseBody()).contains("YES");
    }

    @Test
    void sendCommitTx_204_marksSentNullBody() {
        mockServer.expect(requestTo("http://banka2.local/interbank"))
                .andExpect(method(HttpMethod.POST))
                .andExpect(header("X-Api-Key", "out-222"))
                .andExpect(jsonPath("$.messageType").value("COMMIT_TX"))
                .andExpect(jsonPath("$.message.transactionId.id").value("tx-local-1"))
                .andRespond(withNoContent());

        client.sendCommitTx(222, new ForeignBankId(111, "tx-local-1"));

        mockServer.verify();
        ArgumentCaptor<InterbankMessageEntity> cap = ArgumentCaptor.forClass(InterbankMessageEntity.class);
        verify(msgRepo, times(2)).save(cap.capture());
        assertThat(cap.getAllValues().get(1).getStatus()).isEqualTo(MessageStatus.SENT);
        assertThat(cap.getAllValues().get(1).getHttpStatus()).isEqualTo(204);
        assertThat(cap.getAllValues().get(1).getResponseBody()).isNull();
    }

    @Test
    void sendRollbackTx_204_marksSentNullBody() {
        mockServer.expect(requestTo("http://banka2.local/interbank"))
                .andExpect(method(HttpMethod.POST))
                .andExpect(header("X-Api-Key", "out-222"))
                .andExpect(jsonPath("$.messageType").value("ROLLBACK_TX"))
                .andExpect(jsonPath("$.message.transactionId.id").value("tx-local-1"))
                .andRespond(withNoContent());

        client.sendRollbackTx(222, new ForeignBankId(111, "tx-local-1"));

        mockServer.verify();
        ArgumentCaptor<InterbankMessageEntity> cap = ArgumentCaptor.forClass(InterbankMessageEntity.class);
        verify(msgRepo, times(2)).save(cap.capture());
        assertThat(cap.getAllValues().get(1).getStatus()).isEqualTo(MessageStatus.SENT);
        assertThat(cap.getAllValues().get(1).getHttpStatus()).isEqualTo(204);
    }

    @Test
    void persistOutboundPending_setsPendingStatusAndOurKey() throws Exception {
        // Pristup metodi je package-private — koristimo ga direktno.
        InterbankMessagePayload payload = new InterbankMessagePayload(
                client.newKey(),
                MessageType.NEW_TX,
                mapper.valueToTree(sampleTx()));

        InterbankMessageEntity saved = client.persistOutboundPending(222, payload);

        assertThat(saved.getDirection()).isEqualTo(Direction.OUTBOUND);
        assertThat(saved.getSenderRoutingNumber()).isEqualTo(222);
        assertThat(saved.getStatus()).isEqualTo(MessageStatus.PENDING_SEND);
        assertThat(saved.getLocallyGeneratedKey()).isEqualTo(payload.idempotenceKey().locallyGeneratedKey());
        assertThat(saved.getRetryCount()).isZero();
        assertThat(saved.getRequestBody()).contains("NEW_TX");
        assertThat(saved.getLastAttemptAt()).isNotNull();
        verify(msgRepo, times(1)).save(any(InterbankMessageEntity.class));
    }

    @Test
    void resendByEntity_reusesOriginalKeyAndUpdatesStatus() throws Exception {
        InterbankMessagePayload originalPayload = new InterbankMessagePayload(
                new com.banka1.interbank.protocol.dto.IdempotenceKey(111, "abcd1234abcd1234"),
                MessageType.COMMIT_TX,
                mapper.valueToTree(new com.banka1.interbank.protocol.dto.CommitTransactionBody(
                        new ForeignBankId(111, "tx-local-x"))));
        InterbankMessageEntity entity = InterbankMessageEntity.builder()
                .id(42L)
                .direction(Direction.OUTBOUND)
                .senderRoutingNumber(222)
                .locallyGeneratedKey("abcd1234abcd1234")
                .messageType(MessageType.COMMIT_TX)
                .status(MessageStatus.PENDING_SEND)
                .requestBody(mapper.writeValueAsString(originalPayload))
                .retryCount(1)
                .lastAttemptAt(Instant.now())
                .build();

        mockServer.expect(requestTo("http://banka2.local/interbank"))
                .andExpect(method(HttpMethod.POST))
                // Critical: retry posalje IST locallyGeneratedKey (idempotency invariant).
                .andExpect(content().string(Matchers.containsString("abcd1234abcd1234")))
                .andRespond(withNoContent());

        client.resendByEntity(entity);

        mockServer.verify();
        assertThat(entity.getStatus()).isEqualTo(MessageStatus.SENT);
        assertThat(entity.getHttpStatus()).isEqualTo(204);
    }

    @Test
    void sendNewTx_partnerUnknown_throwsBeforeHttpCall() {
        // Resetuje expectations da bismo izverifikovali da nema HTTP poziva.
        assertThatThrownBy(() -> client.sendNewTx(999, sampleTx()))
                .isInstanceOf(IllegalArgumentException.class)
                .hasMessageContaining("999");
        // mockServer ce baciti AssertionError ako bi se HTTP poziv desio, ali kako smo
        // bacili pre HTTP-a, nije bilo zahteva — verify() bi prosao (nista expected, nista invoked).
    }

    @Test
    void sendNewTx_5xxResponse_propagatesExceptionAndLeavesPendingSend() {
        mockServer.expect(requestTo("http://banka2.local/interbank"))
                .andExpect(method(HttpMethod.POST))
                .andRespond(withServerError().body("partner down"));

        assertThatThrownBy(() -> client.sendNewTx(222, sampleTx()))
                .isInstanceOf(Exception.class);

        mockServer.verify();
        // Persist save desio se samo jednom (PENDING_SEND) — markSent nije pozvan jer je HTTP propustio.
        ArgumentCaptor<InterbankMessageEntity> cap = ArgumentCaptor.forClass(InterbankMessageEntity.class);
        verify(msgRepo, times(1)).save(cap.capture());
        assertThat(cap.getValue().getStatus()).isEqualTo(MessageStatus.PENDING_SEND);
    }
}
