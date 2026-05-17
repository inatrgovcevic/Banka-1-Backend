package com.banka1.interbank.service;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.anyString;
import static org.mockito.Mockito.doThrow;
import static org.mockito.Mockito.never;
import static org.mockito.Mockito.times;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

import com.banka1.interbank.client.BankingCoreInternalClient;
import com.banka1.interbank.client.BankingCoreInternalClient.AccountResolveRes;
import com.banka1.interbank.client.BankingCoreInternalClient.ReserveMonasReq;
import com.banka1.interbank.client.BankingCoreInternalClient.ReserveMonasRes;
import com.banka1.interbank.client.TradingInternalClient;
import com.banka1.interbank.client.TradingInternalClient.ReserveStockReq;
import com.banka1.interbank.client.TradingInternalClient.ReserveStockRes;
import com.banka1.interbank.config.InterbankProperties;
import com.banka1.interbank.model.InterbankNegotiationEntity;
import com.banka1.interbank.model.InterbankTransactionEntity;
import com.banka1.interbank.model.enums.TxStatus;
import com.banka1.interbank.protocol.dto.Asset;
import com.banka1.interbank.protocol.dto.CurrencyCode;
import com.banka1.interbank.protocol.dto.ForeignBankId;
import com.banka1.interbank.protocol.dto.InterbankTransactionPayload;
import com.banka1.interbank.protocol.dto.MonetaryAsset;
import com.banka1.interbank.protocol.dto.MonetaryValue;
import com.banka1.interbank.protocol.dto.NoVoteReason.Reason;
import com.banka1.interbank.protocol.dto.OptionDescription;
import com.banka1.interbank.protocol.dto.Posting;
import com.banka1.interbank.protocol.dto.StockDescription;
import com.banka1.interbank.protocol.dto.TransactionVote;
import com.banka1.interbank.protocol.dto.TxAccount;
import com.banka1.interbank.repository.InterbankContractRepository;
import com.banka1.interbank.repository.InterbankNegotiationRepository;
import com.banka1.interbank.repository.InterbankTransactionRepository;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.datatype.jsr310.JavaTimeModule;
import java.math.BigDecimal;
import java.time.OffsetDateTime;
import java.util.List;
import java.util.Optional;
import java.util.UUID;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.mockito.ArgumentCaptor;
import org.mockito.Mockito;
import org.springframework.web.client.RestClientException;

/**
 * PR_32 Phase 6 Task 6.2-6.6 unit testovi za {@link TransactionExecutorService}.
 *
 * <p>Mockujemo {@link InterbankTransactionRepository},
 * {@link InterbankNegotiationRepository}, {@link InterbankContractRepository},
 * {@link BankingCoreInternalClient}, {@link TradingInternalClient} i
 * {@link InterbankProperties} kako bismo izolovali pure logiku od Spring-a i
 * REST-a. Pun ObjectMapper se koristi (sa JSR310 modulom za OffsetDateTime)
 * radi realnog serijalizovanja Asset/TxAccount sealed interface-a.
 */
class TransactionExecutorServiceTest {

    private static final int MY_ROUTING = 111;
    private static final int PARTNER_ROUTING = 222;

    private TransactionValidator validator;
    private BankingCoreInternalClient bankingCore;
    private TradingInternalClient trading;
    private InterbankProperties props;
    private InterbankTransactionRepository txRepo;
    private InterbankNegotiationRepository negRepo;
    private InterbankContractRepository contractRepo;
    private ObjectMapper mapper;
    private TransactionExecutorService service;

    @BeforeEach
    void setUp() {
        validator = new TransactionValidator();
        bankingCore = Mockito.mock(BankingCoreInternalClient.class);
        trading = Mockito.mock(TradingInternalClient.class);
        props = Mockito.mock(InterbankProperties.class);
        txRepo = Mockito.mock(InterbankTransactionRepository.class);
        negRepo = Mockito.mock(InterbankNegotiationRepository.class);
        contractRepo = Mockito.mock(InterbankContractRepository.class);
        mapper = new ObjectMapper();
        mapper.registerModule(new JavaTimeModule());

        when(props.getMyRoutingNumber()).thenReturn(MY_ROUTING);
        // default — uvek dovoljno sredstava ako ne override-uju test
        when(bankingCore.resolveAccount(anyString())).thenReturn(
                new AccountResolveRes("CLIENT", 42L, CurrencyCode.USD, new BigDecimal("1000000")));
        when(txRepo.save(any())).thenAnswer(inv -> inv.getArgument(0));

        service = new TransactionExecutorService(
                validator, bankingCore, trading, props, txRepo, negRepo, contractRepo, mapper);
    }

    // ===== Helpers ============================================================

    private InterbankTransactionPayload payload(List<Posting> postings) {
        return new InterbankTransactionPayload(
                postings,
                new ForeignBankId(PARTNER_ROUTING, "TX-1"),
                "test", "1234", "289", "Test payment");
    }

    private Posting monas(int routing, String accountSuffix, String amount, CurrencyCode ccy) {
        String num = String.format("%03d%015d", routing, Long.parseLong(accountSuffix));
        return new Posting(
                new TxAccount.Account(num),
                new BigDecimal(amount),
                new Asset.Monas(new MonetaryAsset(ccy)));
    }

    private Posting stockPerson(int routing, String personId, String amount, String ticker) {
        return new Posting(
                new TxAccount.Person(new ForeignBankId(routing, personId)),
                new BigDecimal(amount),
                new Asset.Stock(new StockDescription(ticker)));
    }

    // ===== Test 1: balance fail ===============================================

    @Test
    void balanceFailReturnsNo() {
        var p1 = monas(MY_ROUTING, "1", "-1000", CurrencyCode.USD);
        var p2 = monas(PARTNER_ROUTING, "2", "500", CurrencyCode.USD);
        TransactionVote vote = service.prepareLocal(payload(List.of(p1, p2)));
        assertThat(vote.isYes()).isFalse();
        assertThat(vote.reasons()).hasSize(1);
        assertThat(vote.reasons().get(0).reason()).isEqualTo(Reason.UNBALANCED_TX);
    }

    // ===== Test 2: NO_SUCH_ACCOUNT ============================================

    @Test
    void noSuchAccountVote() {
        var p1 = monas(MY_ROUTING, "1", "-1000", CurrencyCode.USD);
        var p2 = monas(PARTNER_ROUTING, "2", "1000", CurrencyCode.USD);
        when(bankingCore.resolveAccount(anyString()))
                .thenThrow(new RestClientException("not found"));
        TransactionVote vote = service.prepareLocal(payload(List.of(p1, p2)));
        assertThat(vote.isYes()).isFalse();
        assertThat(vote.reasons()).extracting(r -> r.reason())
                .contains(Reason.NO_SUCH_ACCOUNT);
    }

    // ===== Test 3: INSUFFICIENT_ASSET =========================================

    @Test
    void insufficientAssetVote() {
        var p1 = monas(MY_ROUTING, "1", "-1000", CurrencyCode.USD);
        var p2 = monas(PARTNER_ROUTING, "2", "1000", CurrencyCode.USD);
        when(bankingCore.resolveAccount(anyString())).thenReturn(
                new AccountResolveRes("CLIENT", 42L, CurrencyCode.USD, new BigDecimal("50")));
        TransactionVote vote = service.prepareLocal(payload(List.of(p1, p2)));
        assertThat(vote.isYes()).isFalse();
        assertThat(vote.reasons()).extracting(r -> r.reason())
                .contains(Reason.INSUFFICIENT_ASSET);
    }

    // ===== Test 4: UNACCEPTABLE_ASSET =========================================

    @Test
    void unacceptableAssetVote() {
        // STOCK na MONAS-style Account → UNACCEPTABLE_ASSET
        var bogus = new Posting(
                new TxAccount.Account("111000000000000001"),
                new BigDecimal("-5"),
                new Asset.Stock(new StockDescription("AAPL")));
        // counter-side da balansiramo asset key (STOCK:AAPL)
        var counter = stockPerson(PARTNER_ROUTING, "C-9", "5", "AAPL");
        TransactionVote vote = service.prepareLocal(payload(List.of(bogus, counter)));
        assertThat(vote.isYes()).isFalse();
        assertThat(vote.reasons()).extracting(r -> r.reason())
                .contains(Reason.UNACCEPTABLE_ASSET);
    }

    // ===== Test 5: OPTION_AMOUNT_INCORRECT ====================================

    @Test
    void optionAmountIncorrectVote() {
        var optDesc = new OptionDescription(
                new ForeignBankId(PARTNER_ROUTING, "NEG-X"),
                new StockDescription("AAPL"),
                new MonetaryValue(CurrencyCode.USD, new BigDecimal("100")),
                OffsetDateTime.now().plusDays(30),
                10);
        // posting amount = 42 — niti k (=10) niti k*pi (=1000)
        Posting ours = new Posting(
                new TxAccount.Option(new ForeignBankId(MY_ROUTING, "OPT-1")),
                new BigDecimal("-42"),
                new Asset.Option(optDesc));
        Posting counter = new Posting(
                new TxAccount.Option(new ForeignBankId(PARTNER_ROUTING, "OPT-2")),
                new BigDecimal("42"),
                new Asset.Option(optDesc));
        TransactionVote vote = service.prepareLocal(payload(List.of(ours, counter)));
        assertThat(vote.isYes()).isFalse();
        assertThat(vote.reasons()).extracting(r -> r.reason())
                .contains(Reason.OPTION_AMOUNT_INCORRECT);
    }

    // ===== Test 6: OPTION_USED_OR_EXPIRED =====================================

    @Test
    void optionUsedOrExpiredVote() {
        var negId = new ForeignBankId(PARTNER_ROUTING, "NEG-EXP");
        var optDesc = new OptionDescription(
                negId,
                new StockDescription("AAPL"),
                new MonetaryValue(CurrencyCode.USD, new BigDecimal("100")),
                OffsetDateTime.now().plusDays(30),
                10);
        Posting ours = new Posting(
                new TxAccount.Option(new ForeignBankId(MY_ROUTING, "OPT-1")),
                new BigDecimal("-10"),
                new Asset.Option(optDesc));
        Posting counter = new Posting(
                new TxAccount.Option(new ForeignBankId(PARTNER_ROUTING, "OPT-2")),
                new BigDecimal("10"),
                new Asset.Option(optDesc));

        // Mirror je u isOngoing=false (vec iskoriscena ili expirirala)
        InterbankNegotiationEntity neg = InterbankNegotiationEntity.builder()
                .id(negId.id())
                .isOngoing(false)
                .settlementDate(OffsetDateTime.now().plusDays(30))
                .build();
        when(negRepo.findById(negId.id())).thenReturn(Optional.of(neg));

        TransactionVote vote = service.prepareLocal(payload(List.of(ours, counter)));
        assertThat(vote.isYes()).isFalse();
        assertThat(vote.reasons()).extracting(r -> r.reason())
                .contains(Reason.OPTION_USED_OR_EXPIRED);
    }

    // ===== Test 7: OPTION_NEGOTIATION_NOT_FOUND ===============================

    @Test
    void optionNegotiationNotFoundVote() {
        var negId = new ForeignBankId(PARTNER_ROUTING, "NEG-GHOST");
        var optDesc = new OptionDescription(
                negId,
                new StockDescription("AAPL"),
                new MonetaryValue(CurrencyCode.USD, new BigDecimal("100")),
                OffsetDateTime.now().plusDays(30),
                10);
        Posting ours = new Posting(
                new TxAccount.Option(new ForeignBankId(MY_ROUTING, "OPT-1")),
                new BigDecimal("-10"),
                new Asset.Option(optDesc));
        Posting counter = new Posting(
                new TxAccount.Option(new ForeignBankId(PARTNER_ROUTING, "OPT-2")),
                new BigDecimal("10"),
                new Asset.Option(optDesc));
        when(negRepo.findById(negId.id())).thenReturn(Optional.empty());

        TransactionVote vote = service.prepareLocal(payload(List.of(ours, counter)));
        assertThat(vote.isYes()).isFalse();
        assertThat(vote.reasons()).extracting(r -> r.reason())
                .contains(Reason.OPTION_NEGOTIATION_NOT_FOUND);
    }

    // ===== Test 8: prepare happy path =========================================

    @Test
    void prepareHappyPath() {
        var p1 = monas(MY_ROUTING, "1", "-1000", CurrencyCode.USD);
        var p2 = monas(PARTNER_ROUTING, "2", "1000", CurrencyCode.USD);
        UUID reservationId = UUID.randomUUID();
        when(bankingCore.reserveMonas(any(ReserveMonasReq.class)))
                .thenReturn(new ReserveMonasRes(reservationId));

        TransactionVote vote = service.prepareLocal(payload(List.of(p1, p2)));
        assertThat(vote.isYes()).isTrue();
        verify(bankingCore, times(1)).reserveMonas(any());

        ArgumentCaptor<InterbankTransactionEntity> captor =
                ArgumentCaptor.forClass(InterbankTransactionEntity.class);
        verify(txRepo).save(captor.capture());
        InterbankTransactionEntity persisted = captor.getValue();
        assertThat(persisted.getStatus()).isEqualTo(TxStatus.PREPARED);
        assertThat(persisted.getTransactionIdRouting()).isEqualTo(PARTNER_ROUTING);
        assertThat(persisted.getTransactionIdLocal()).isEqualTo("TX-1");
        assertThat(persisted.getReservationRefs()).contains(reservationId.toString());
    }

    // ===== Test 9: commit idempotent ==========================================

    @Test
    void commitIdempotent() throws Exception {
        UUID reservationId = UUID.randomUUID();
        String refsJson = mapper.writeValueAsString(List.of(
                java.util.Map.of("type", "MONAS", "id", reservationId.toString())));
        InterbankTransactionEntity entity = InterbankTransactionEntity.builder()
                .transactionIdRouting(PARTNER_ROUTING)
                .transactionIdLocal("TX-1")
                .status(TxStatus.PREPARED)
                .reservationRefs(refsJson)
                .build();

        when(txRepo.findByTransactionIdRoutingAndTransactionIdLocal(PARTNER_ROUTING, "TX-1"))
                .thenAnswer(inv -> Optional.of(entity));

        ForeignBankId txId = new ForeignBankId(PARTNER_ROUTING, "TX-1");
        service.commitLocal(txId);
        assertThat(entity.getStatus()).isEqualTo(TxStatus.COMMITTED);
        verify(bankingCore, times(1)).commitMonas(reservationId);

        // Second call — vec COMMITTED, ne sme da ponovo zove commitMonas
        service.commitLocal(txId);
        verify(bankingCore, times(1)).commitMonas(reservationId);
    }

    // ===== Test 10: rollback idempotent =======================================

    @Test
    void rollbackIdempotent() throws Exception {
        UUID reservationId = UUID.randomUUID();
        String refsJson = mapper.writeValueAsString(List.of(
                java.util.Map.of("type", "MONAS", "id", reservationId.toString())));
        InterbankTransactionEntity entity = InterbankTransactionEntity.builder()
                .transactionIdRouting(PARTNER_ROUTING)
                .transactionIdLocal("TX-1")
                .status(TxStatus.PREPARED)
                .reservationRefs(refsJson)
                .build();

        when(txRepo.findByTransactionIdRoutingAndTransactionIdLocal(PARTNER_ROUTING, "TX-1"))
                .thenAnswer(inv -> Optional.of(entity));

        ForeignBankId txId = new ForeignBankId(PARTNER_ROUTING, "TX-1");
        service.rollbackLocal(txId);
        assertThat(entity.getStatus()).isEqualTo(TxStatus.ROLLED_BACK);
        verify(bankingCore, times(1)).releaseMonas(reservationId);

        // Second call — vec ROLLED_BACK, ne sme da ponovo zove releaseMonas
        service.rollbackLocal(txId);
        verify(bankingCore, times(1)).releaseMonas(reservationId);
    }

    // ===== Test 11: compensate on reservation failure =========================

    @Test
    void compensateOnReservationFail() {
        // Dva nasa credit postings: prvi prolazi, drugi pada → kompenzacija na prvi
        var p1Ours = monas(MY_ROUTING, "1", "-500", CurrencyCode.USD);
        var p1Partner = monas(PARTNER_ROUTING, "1", "500", CurrencyCode.USD);
        var p2Ours = monas(MY_ROUTING, "2", "-300", CurrencyCode.EUR);
        var p2Partner = monas(PARTNER_ROUTING, "2", "300", CurrencyCode.EUR);

        UUID firstRes = UUID.randomUUID();
        when(bankingCore.reserveMonas(any(ReserveMonasReq.class)))
                .thenReturn(new ReserveMonasRes(firstRes))
                .thenThrow(new RestClientException("ledger lock"));

        assertThatThrownBy(() ->
                service.prepareLocal(payload(List.of(p1Ours, p1Partner, p2Ours, p2Partner))))
                .isInstanceOf(InterbankException.class);

        // Prvi mora biti oslobodjen (kompenzacija unazad)
        verify(bankingCore, times(1)).releaseMonas(firstRes);
        // Nikad nije stigao do persist
        verify(txRepo, never()).save(any());
    }

    // ===== Test 12: commit terminal state throws AlreadyCommittedException ===

    @Test
    void commitFromRolledBackThrows() {
        InterbankTransactionEntity entity = InterbankTransactionEntity.builder()
                .transactionIdRouting(PARTNER_ROUTING)
                .transactionIdLocal("TX-1")
                .status(TxStatus.ROLLED_BACK)
                .reservationRefs("[]")
                .build();
        when(txRepo.findByTransactionIdRoutingAndTransactionIdLocal(PARTNER_ROUTING, "TX-1"))
                .thenAnswer(inv -> Optional.of(entity));
        ForeignBankId txId = new ForeignBankId(PARTNER_ROUTING, "TX-1");
        assertThatThrownBy(() -> service.commitLocal(txId))
                .isInstanceOf(AlreadyCommittedException.class);
    }

    // ===== Test 13: unknown tx commit/rollback is no-op =======================

    @Test
    void commitUnknownTxIsNoOp() {
        when(txRepo.findByTransactionIdRoutingAndTransactionIdLocal(PARTNER_ROUTING, "TX-XX"))
                .thenReturn(Optional.empty());
        service.commitLocal(new ForeignBankId(PARTNER_ROUTING, "TX-XX"));
        verify(bankingCore, never()).commitMonas(any());
        verify(txRepo, never()).save(any());
    }

    @Test
    void rollbackUnknownTxIsNoOp() {
        when(txRepo.findByTransactionIdRoutingAndTransactionIdLocal(PARTNER_ROUTING, "TX-XX"))
                .thenReturn(Optional.empty());
        service.rollbackLocal(new ForeignBankId(PARTNER_ROUTING, "TX-XX"));
        verify(bankingCore, never()).releaseMonas(any());
        verify(txRepo, never()).save(any());
    }

    // ===== Test 14: commit MONAS failure flips to FAILED =====================

    @Test
    void commitFailureFlipsToFailedStatus() throws Exception {
        UUID reservationId = UUID.randomUUID();
        String refsJson = mapper.writeValueAsString(List.of(
                java.util.Map.of("type", "MONAS", "id", reservationId.toString())));
        InterbankTransactionEntity entity = InterbankTransactionEntity.builder()
                .transactionIdRouting(PARTNER_ROUTING)
                .transactionIdLocal("TX-1")
                .status(TxStatus.PREPARED)
                .reservationRefs(refsJson)
                .build();
        when(txRepo.findByTransactionIdRoutingAndTransactionIdLocal(PARTNER_ROUTING, "TX-1"))
                .thenAnswer(inv -> Optional.of(entity));
        doThrow(new RestClientException("ledger down"))
                .when(bankingCore).commitMonas(reservationId);

        assertThatThrownBy(() ->
                service.commitLocal(new ForeignBankId(PARTNER_ROUTING, "TX-1")))
                .isInstanceOf(InterbankException.class);
        assertThat(entity.getStatus()).isEqualTo(TxStatus.FAILED);
    }
}
