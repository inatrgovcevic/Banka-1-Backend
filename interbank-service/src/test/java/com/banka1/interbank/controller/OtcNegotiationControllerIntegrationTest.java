package com.banka1.interbank.controller;

import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.Mockito.doNothing;
import static org.mockito.Mockito.doThrow;
import static org.mockito.Mockito.never;
import static org.mockito.Mockito.times;
import static org.mockito.Mockito.verify;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.delete;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.get;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.post;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.put;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.jsonPath;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.status;

import com.banka1.interbank.model.InterbankNegotiationEntity;
import com.banka1.interbank.otc.dto.OtcOfferDto;
import com.banka1.interbank.protocol.dto.CurrencyCode;
import com.banka1.interbank.protocol.dto.ForeignBankId;
import com.banka1.interbank.protocol.dto.MonetaryValue;
import com.banka1.interbank.protocol.dto.StockDescription;
import com.banka1.interbank.repository.InterbankNegotiationRepository;
import com.banka1.interbank.client.TradingInternalClient;
import com.banka1.interbank.client.UserInternalClient;
import com.banka1.interbank.service.InterbankCoordinatorService;
import com.banka1.interbank.service.InterbankException;
import com.banka1.interbank.service.TransactionExecutorService;
import com.fasterxml.jackson.databind.ObjectMapper;
import java.math.BigDecimal;
import java.time.OffsetDateTime;
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
 * PR_32 Phase 10 Task 10.5 integration testovi za {@link OtcNegotiationController}.
 *
 * <p>Pokriva Tim 2 §3.2-3.6 + §6.3 response code matrix:
 * <ul>
 *   <li>createNegotiationHappyPath → 200 OK + ForeignBankId</li>
 *   <li>createNegotiationInvalidSeller → 400</li>
 *   <li>createNegotiationPastSettlement → 400</li>
 *   <li>counterOfferHappy → 204</li>
 *   <li><strong>counterOfferTurnViolation → 409</strong> (Tim 2 §6.3 KRITICNO)</li>
 *   <li><strong>counterOfferClosed → 409</strong></li>
 *   <li>counterOfferNotFound → 404</li>
 *   <li>getNegotiationHappy → 200 + DTO</li>
 *   <li>getNegotiationNotFound → 404</li>
 *   <li>deleteNegotiationHappy → 204</li>
 *   <li>deleteNegotiationNotFound → 404</li>
 *   <li>acceptNegotiationHappyPath → 204 (mock coordinator)</li>
 *   <li>acceptTurnViolation → 409</li>
 *   <li>acceptClosed → 409</li>
 *   <li>acceptCoordinatorFailure → 500 (InterbankException)</li>
 * </ul>
 *
 * <p>{@link InterbankCoordinatorService} je {@link MockitoBean} jer ne zelimo
 * stvarno 2PC na trading-service / banking-core. Auth filter ostaje aktivan
 * preko {@code springSecurity()} apply-a.
 */
@SpringBootTest
@ActiveProfiles("test")
class OtcNegotiationControllerIntegrationTest {

    private static final String VALID_TOKEN = "test-inbound";
    private static final int PARTNER_ROUTING = 222;
    private static final int MY_ROUTING = 111;

    @Autowired private WebApplicationContext context;
    @Autowired private ObjectMapper mapper;
    @Autowired private InterbankNegotiationRepository negRepo;

    @MockitoBean private InterbankCoordinatorService coordinator;
    @MockitoBean private TransactionExecutorService executor;
    @MockitoBean private TradingInternalClient trading;
    @MockitoBean private UserInternalClient userClient;
    // PR_33 Phase A: vidi komentar u InterbankInboundControllerIntegrationTest.
    @MockitoBean private com.banka1.interbank.service.InterbankClient interbankClient;

    private MockMvc mockMvc;

    @BeforeEach
    void setUp() {
        mockMvc = MockMvcBuilders
                .webAppContextSetup(context)
                .apply(org.springframework.security.test.web.servlet.setup.SecurityMockMvcConfigurers.springSecurity())
                .build();
        negRepo.deleteAll();
    }

    // ===== Helpers ============================================================

    private OtcOfferDto sampleOffer(ForeignBankId buyer, ForeignBankId seller, ForeignBankId lastBy,
                                    OffsetDateTime settle) {
        return new OtcOfferDto(
                new StockDescription("AAPL"),
                settle,
                new MonetaryValue(CurrencyCode.USD, new BigDecimal("150.00")),
                new MonetaryValue(CurrencyCode.USD, new BigDecimal("5.00")),
                buyer,
                seller,
                100,
                lastBy
        );
    }

    private InterbankNegotiationEntity persistedNegotiation(String id, int lastByRouting, String lastById,
                                                            boolean ongoing) {
        OffsetDateTime settle = OffsetDateTime.now().plusDays(30);
        InterbankNegotiationEntity e = InterbankNegotiationEntity.builder()
                .id(id)
                .buyerRoutingNumber(PARTNER_ROUTING)
                .buyerId("C-1001")
                .sellerRoutingNumber(MY_ROUTING)
                .sellerId("C-2002")
                .stockTicker("AAPL")
                .amount(100)
                .priceCurrency("USD")
                .priceAmount(new BigDecimal("150.00"))
                .premiumCurrency("USD")
                .premiumAmount(new BigDecimal("5.00"))
                .settlementDate(settle)
                .lastModifiedByRouting(lastByRouting)
                .lastModifiedById(lastById)
                .isOngoing(ongoing)
                .isAuthoritative(true)
                .build();
        return negRepo.save(e);
    }

    // ===== Create (§3.2) ======================================================

    @Test
    void createNegotiationHappyPath() throws Exception {
        ForeignBankId buyer = new ForeignBankId(PARTNER_ROUTING, "C-1001");
        ForeignBankId seller = new ForeignBankId(MY_ROUTING, "C-2002");
        OtcOfferDto offer = sampleOffer(buyer, seller, buyer, OffsetDateTime.now().plusDays(30));

        mockMvc.perform(post("/negotiations")
                        .header("X-Api-Key", VALID_TOKEN)
                        .contentType(MediaType.APPLICATION_JSON)
                        .content(mapper.writeValueAsString(offer)))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.routingNumber").value(MY_ROUTING))
                .andExpect(jsonPath("$.id").exists());

        assertThat(negRepo.findAll()).hasSize(1);
        var saved = negRepo.findAll().get(0);
        assertThat(saved.isOngoing()).isTrue();
        assertThat(saved.isAuthoritative()).isTrue();
        assertThat(saved.getStockTicker()).isEqualTo("AAPL");
    }

    @Test
    void createNegotiationInvalidSeller() throws Exception {
        // sellerId.routingNumber=999 (not MY_ROUTING) → 400
        ForeignBankId buyer = new ForeignBankId(PARTNER_ROUTING, "C-1001");
        ForeignBankId seller = new ForeignBankId(999, "C-XXX");
        OtcOfferDto offer = sampleOffer(buyer, seller, buyer, OffsetDateTime.now().plusDays(30));

        mockMvc.perform(post("/negotiations")
                        .header("X-Api-Key", VALID_TOKEN)
                        .contentType(MediaType.APPLICATION_JSON)
                        .content(mapper.writeValueAsString(offer)))
                .andExpect(status().isBadRequest())
                .andExpect(jsonPath("$.error").value(org.hamcrest.Matchers.containsString("sellerId.routingNumber")));
    }

    @Test
    void createNegotiationPastSettlement() throws Exception {
        ForeignBankId buyer = new ForeignBankId(PARTNER_ROUTING, "C-1001");
        ForeignBankId seller = new ForeignBankId(MY_ROUTING, "C-2002");
        OtcOfferDto offer = sampleOffer(buyer, seller, buyer, OffsetDateTime.now().minusDays(1));

        mockMvc.perform(post("/negotiations")
                        .header("X-Api-Key", VALID_TOKEN)
                        .contentType(MediaType.APPLICATION_JSON)
                        .content(mapper.writeValueAsString(offer)))
                .andExpect(status().isBadRequest())
                .andExpect(jsonPath("$.error").value(org.hamcrest.Matchers.containsString("settlementDate")));
    }

    @Test
    void createNegotiationBuyerRoutingMismatch() throws Exception {
        // buyerId.routingNumber != X-Api-Key sender (PARTNER_ROUTING) → 400
        ForeignBankId buyer = new ForeignBankId(333, "C-1001");
        ForeignBankId seller = new ForeignBankId(MY_ROUTING, "C-2002");
        OtcOfferDto offer = sampleOffer(buyer, seller, buyer, OffsetDateTime.now().plusDays(30));

        mockMvc.perform(post("/negotiations")
                        .header("X-Api-Key", VALID_TOKEN)
                        .contentType(MediaType.APPLICATION_JSON)
                        .content(mapper.writeValueAsString(offer)))
                .andExpect(status().isBadRequest())
                .andExpect(jsonPath("$.error").value(org.hamcrest.Matchers.containsString("buyerId.routingNumber")));
    }

    // ===== Update (§3.3) — KRITICNO 409 za turn/closed ========================

    @Test
    void counterOfferHappy() throws Exception {
        // Initial state: lastModifiedBy=MY_ROUTING/C-2002 (seller, we counter-offered)
        // → partner buyer can counter back
        persistedNegotiation("neg-1", MY_ROUTING, "C-2002", true);

        ForeignBankId buyer = new ForeignBankId(PARTNER_ROUTING, "C-1001");
        ForeignBankId seller = new ForeignBankId(MY_ROUTING, "C-2002");
        OtcOfferDto offer = new OtcOfferDto(
                new StockDescription("AAPL"),
                OffsetDateTime.now().plusDays(30),
                new MonetaryValue(CurrencyCode.USD, new BigDecimal("145.00")),
                new MonetaryValue(CurrencyCode.USD, new BigDecimal("4.00")),
                buyer, seller, 100,
                buyer  // last modified BY buyer (partner)
        );

        mockMvc.perform(put("/negotiations/{rn}/{id}", MY_ROUTING, "neg-1")
                        .header("X-Api-Key", VALID_TOKEN)
                        .contentType(MediaType.APPLICATION_JSON)
                        .content(mapper.writeValueAsString(offer)))
                .andExpect(status().isNoContent());

        var updated = negRepo.findById("neg-1").orElseThrow();
        assertThat(updated.getPriceAmount()).isEqualByComparingTo("145.00");
        assertThat(updated.getLastModifiedByRouting()).isEqualTo(PARTNER_ROUTING);
    }

    @Test
    void counterOfferTurnViolation() throws Exception {
        // Initial state: lastModifiedBy was buyer (partner) → buyer cannot
        // counter-offer again. Tim 2 §6.3 → 409 Conflict (NOT 400).
        persistedNegotiation("neg-2", PARTNER_ROUTING, "C-1001", true);

        ForeignBankId buyer = new ForeignBankId(PARTNER_ROUTING, "C-1001");
        ForeignBankId seller = new ForeignBankId(MY_ROUTING, "C-2002");
        OtcOfferDto offer = new OtcOfferDto(
                new StockDescription("AAPL"),
                OffsetDateTime.now().plusDays(30),
                new MonetaryValue(CurrencyCode.USD, new BigDecimal("145.00")),
                new MonetaryValue(CurrencyCode.USD, new BigDecimal("4.00")),
                buyer, seller, 100,
                buyer
        );

        mockMvc.perform(put("/negotiations/{rn}/{id}", MY_ROUTING, "neg-2")
                        .header("X-Api-Key", VALID_TOKEN)
                        .contentType(MediaType.APPLICATION_JSON)
                        .content(mapper.writeValueAsString(offer)))
                .andExpect(status().isConflict())
                .andExpect(jsonPath("$.error").value(org.hamcrest.Matchers.containsString("turn")));
    }

    @Test
    void counterOfferClosed() throws Exception {
        // Negotiation already closed → 409
        persistedNegotiation("neg-3", MY_ROUTING, "C-2002", false);

        ForeignBankId buyer = new ForeignBankId(PARTNER_ROUTING, "C-1001");
        ForeignBankId seller = new ForeignBankId(MY_ROUTING, "C-2002");
        OtcOfferDto offer = new OtcOfferDto(
                new StockDescription("AAPL"),
                OffsetDateTime.now().plusDays(30),
                new MonetaryValue(CurrencyCode.USD, new BigDecimal("145.00")),
                new MonetaryValue(CurrencyCode.USD, new BigDecimal("4.00")),
                buyer, seller, 100,
                buyer
        );

        mockMvc.perform(put("/negotiations/{rn}/{id}", MY_ROUTING, "neg-3")
                        .header("X-Api-Key", VALID_TOKEN)
                        .contentType(MediaType.APPLICATION_JSON)
                        .content(mapper.writeValueAsString(offer)))
                .andExpect(status().isConflict())
                .andExpect(jsonPath("$.error").value(org.hamcrest.Matchers.containsString("closed")));
    }

    @Test
    void counterOfferMalformedSettlement() throws Exception {
        persistedNegotiation("neg-4", MY_ROUTING, "C-2002", true);

        ForeignBankId buyer = new ForeignBankId(PARTNER_ROUTING, "C-1001");
        ForeignBankId seller = new ForeignBankId(MY_ROUTING, "C-2002");
        OtcOfferDto offer = new OtcOfferDto(
                new StockDescription("AAPL"),
                OffsetDateTime.now().minusDays(1),  // PAST settlement
                new MonetaryValue(CurrencyCode.USD, new BigDecimal("145.00")),
                new MonetaryValue(CurrencyCode.USD, new BigDecimal("4.00")),
                buyer, seller, 100,
                buyer
        );

        mockMvc.perform(put("/negotiations/{rn}/{id}", MY_ROUTING, "neg-4")
                        .header("X-Api-Key", VALID_TOKEN)
                        .contentType(MediaType.APPLICATION_JSON)
                        .content(mapper.writeValueAsString(offer)))
                .andExpect(status().isBadRequest());
    }

    @Test
    void counterOfferNotFound() throws Exception {
        ForeignBankId buyer = new ForeignBankId(PARTNER_ROUTING, "C-1001");
        ForeignBankId seller = new ForeignBankId(MY_ROUTING, "C-2002");
        OtcOfferDto offer = new OtcOfferDto(
                new StockDescription("AAPL"),
                OffsetDateTime.now().plusDays(30),
                new MonetaryValue(CurrencyCode.USD, new BigDecimal("145.00")),
                new MonetaryValue(CurrencyCode.USD, new BigDecimal("4.00")),
                buyer, seller, 100,
                buyer
        );

        mockMvc.perform(put("/negotiations/{rn}/{id}", MY_ROUTING, "does-not-exist")
                        .header("X-Api-Key", VALID_TOKEN)
                        .contentType(MediaType.APPLICATION_JSON)
                        .content(mapper.writeValueAsString(offer)))
                .andExpect(status().isNotFound());
    }

    // ===== Get (§3.4) =========================================================

    @Test
    void getNegotiationHappy() throws Exception {
        persistedNegotiation("neg-5", MY_ROUTING, "C-2002", true);

        mockMvc.perform(get("/negotiations/{rn}/{id}", MY_ROUTING, "neg-5")
                        .header("X-Api-Key", VALID_TOKEN))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.stock.ticker").value("AAPL"))
                .andExpect(jsonPath("$.amount").value(100))
                .andExpect(jsonPath("$.isOngoing").value(true));
    }

    @Test
    void getNegotiationNotFound() throws Exception {
        mockMvc.perform(get("/negotiations/{rn}/{id}", MY_ROUTING, "nope")
                        .header("X-Api-Key", VALID_TOKEN))
                .andExpect(status().isNotFound());
    }

    // ===== Delete (§3.5) ======================================================

    @Test
    void deleteNegotiationHappy() throws Exception {
        persistedNegotiation("neg-6", MY_ROUTING, "C-2002", true);

        mockMvc.perform(delete("/negotiations/{rn}/{id}", MY_ROUTING, "neg-6")
                        .header("X-Api-Key", VALID_TOKEN))
                .andExpect(status().isNoContent());

        var closed = negRepo.findById("neg-6").orElseThrow();
        assertThat(closed.isOngoing()).isFalse();
    }

    @Test
    void deleteNegotiationNotFound() throws Exception {
        mockMvc.perform(delete("/negotiations/{rn}/{id}", MY_ROUTING, "nope")
                        .header("X-Api-Key", VALID_TOKEN))
                .andExpect(status().isNotFound());
    }

    // ===== Accept (§3.6) ======================================================

    @Test
    void acceptNegotiationHappyPath() throws Exception {
        // lastModifiedBy = seller (MY_ROUTING) so buyer (partner) can accept
        persistedNegotiation("neg-7", MY_ROUTING, "C-2002", true);
        doNothing().when(coordinator).acceptNegotiation(any(InterbankNegotiationEntity.class));

        mockMvc.perform(get("/negotiations/{rn}/{id}/accept", MY_ROUTING, "neg-7")
                        .header("X-Api-Key", VALID_TOKEN))
                .andExpect(status().isNoContent());

        verify(coordinator, times(1)).acceptNegotiation(any(InterbankNegotiationEntity.class));
    }

    @Test
    void acceptTurnViolation() throws Exception {
        // lastModifiedBy = buyer (PARTNER_ROUTING) → buyer cannot accept own offer
        persistedNegotiation("neg-8", PARTNER_ROUTING, "C-1001", true);

        mockMvc.perform(get("/negotiations/{rn}/{id}/accept", MY_ROUTING, "neg-8")
                        .header("X-Api-Key", VALID_TOKEN))
                .andExpect(status().isConflict())
                .andExpect(jsonPath("$.error").value(org.hamcrest.Matchers.containsString("turn")));

        verify(coordinator, never()).acceptNegotiation(any(InterbankNegotiationEntity.class));
    }

    @Test
    void acceptClosed() throws Exception {
        persistedNegotiation("neg-9", MY_ROUTING, "C-2002", false);

        mockMvc.perform(get("/negotiations/{rn}/{id}/accept", MY_ROUTING, "neg-9")
                        .header("X-Api-Key", VALID_TOKEN))
                .andExpect(status().isConflict())
                .andExpect(jsonPath("$.error").value(org.hamcrest.Matchers.containsString("closed")));

        verify(coordinator, never()).acceptNegotiation(any(InterbankNegotiationEntity.class));
    }

    @Test
    void acceptCoordinatorFailureReturns500() throws Exception {
        persistedNegotiation("neg-10", MY_ROUTING, "C-2002", true);
        doThrow(new InterbankException("Partner rejected: insufficient balance"))
                .when(coordinator).acceptNegotiation(any(InterbankNegotiationEntity.class));

        mockMvc.perform(get("/negotiations/{rn}/{id}/accept", MY_ROUTING, "neg-10")
                        .header("X-Api-Key", VALID_TOKEN))
                .andExpect(status().isInternalServerError())
                .andExpect(jsonPath("$.error").value(org.hamcrest.Matchers.containsString("Partner rejected")));
    }

    @Test
    void acceptNotFound() throws Exception {
        mockMvc.perform(get("/negotiations/{rn}/{id}/accept", MY_ROUTING, "missing")
                        .header("X-Api-Key", VALID_TOKEN))
                .andExpect(status().isNotFound());
    }

    @Test
    void missingApiKeyReturns401() throws Exception {
        mockMvc.perform(get("/negotiations/{rn}/{id}", MY_ROUTING, "neg-x"))
                .andExpect(status().isUnauthorized());
    }
}
