package com.banka1.interbank.otc.wrapper;

import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.anyInt;
import static org.mockito.ArgumentMatchers.eq;
import static org.mockito.Mockito.never;
import static org.mockito.Mockito.times;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;
import static org.springframework.security.test.web.servlet.request.SecurityMockMvcRequestPostProcessors.jwt;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.delete;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.get;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.post;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.put;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.jsonPath;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.status;

import com.banka1.interbank.client.TradingInternalClient;
import com.banka1.interbank.client.UserInternalClient;
import com.banka1.interbank.model.InterbankNegotiationEntity;
import com.banka1.interbank.otc.dto.OtcOfferDto;
import com.banka1.interbank.otc.wrapper.dto.OutboundCounterOfferRequest;
import com.banka1.interbank.otc.wrapper.dto.OutboundCreateNegotiationRequest;
import com.banka1.interbank.protocol.dto.CurrencyCode;
import com.banka1.interbank.protocol.dto.ForeignBankId;
import com.banka1.interbank.repository.InterbankNegotiationRepository;
import com.banka1.interbank.service.InterbankClient;
import com.banka1.interbank.service.InterbankCoordinatorService;
import com.banka1.interbank.service.TransactionExecutorService;
import com.fasterxml.jackson.databind.ObjectMapper;
import java.math.BigDecimal;
import java.time.OffsetDateTime;
import java.util.List;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.test.context.SpringBootTest;
import org.springframework.http.MediaType;
import org.springframework.http.ResponseEntity;
import org.springframework.test.context.ActiveProfiles;
import org.springframework.test.context.bean.override.mockito.MockitoBean;
import org.springframework.test.web.servlet.MockMvc;
import org.springframework.test.web.servlet.setup.MockMvcBuilders;
import org.springframework.web.context.WebApplicationContext;

/**
 * PR_33 Phase A integration testovi za {@link InterbankOtcOutboundController}.
 *
 * <p>Pokriva sve 6 rute + edge cases:
 * <ul>
 *   <li>POST /api/interbank/otc/negotiations — happy path, validacija
 *       seller=mi (400)</li>
 *   <li>GET /api/interbank/otc/negotiations — filter po user-u</li>
 *   <li>GET /api/interbank/otc/negotiations/{id} — by local id i remote id</li>
 *   <li>PUT /api/interbank/otc/negotiations/{id}/counter — 204 partner happy
 *       + 409 turn violation</li>
 *   <li>POST /api/interbank/otc/negotiations/{id}/accept — 204 partner happy</li>
 *   <li>DELETE /api/interbank/otc/negotiations/{id} — 204</li>
 * </ul>
 *
 * <p>{@link InterbankClient} je {@link MockitoBean} — ne pravimo stvarne
 * HTTP pozive ka partner-u. JWT mock se postavlja kroz
 * {@code SecurityMockMvcRequestPostProcessors.jwt()} sa OTC_TRADE authority
 * + numeric {@code id} claim.
 */
@SpringBootTest
@ActiveProfiles("test")
class InterbankOtcOutboundControllerIntegrationTest {

    private static final int MY_ROUTING = 111;
    private static final int PARTNER_ROUTING = 222;
    private static final Long MILE_ID = 15L;  // Mile Interbank (C-15)

    @Autowired private WebApplicationContext context;
    @Autowired private ObjectMapper mapper;
    @Autowired private InterbankNegotiationRepository negRepo;

    @MockitoBean private InterbankClient interbankClient;
    @MockitoBean private InterbankCoordinatorService coordinator;
    @MockitoBean private TransactionExecutorService executor;
    @MockitoBean private TradingInternalClient trading;
    @MockitoBean private UserInternalClient userClient;

    private MockMvc mockMvc;

    @BeforeEach
    void setUp() {
        mockMvc = MockMvcBuilders
                .webAppContextSetup(context)
                .apply(org.springframework.security.test.web.servlet.setup.SecurityMockMvcConfigurers
                        .springSecurity())
                .build();
        negRepo.deleteAll();
    }

    /**
     * JWT mock processor sa OTC_TRADE authority i {@code id} claim = Mile.
     */
    private static org.springframework.test.web.servlet.request.RequestPostProcessor mileJwt() {
        return jwt().jwt(j -> j.claim("id", MILE_ID).claim("name", "Mile Interbank")
                .claim("roles", List.of("CLIENT_TRADING"))
                .claim("permissions", List.of("OTC_TRADE", "BANKING_BASIC")))
                .authorities(
                        new org.springframework.security.core.authority.SimpleGrantedAuthority("OTC_TRADE"),
                        new org.springframework.security.core.authority.SimpleGrantedAuthority("ROLE_CLIENT_TRADING"));
    }

    private OutboundCreateNegotiationRequest sampleCreateRequest(int sellerRouting) {
        return new OutboundCreateNegotiationRequest(
                "AAPL",
                OffsetDateTime.now().plusDays(30),
                CurrencyCode.USD,
                new BigDecimal("150.00"),
                CurrencyCode.USD,
                new BigDecimal("5.00"),
                new ForeignBankId(sellerRouting, "C-6"),  // Tim 2 Mile Interbank (C-6)
                100,
                null  // buyerLocalUserId — vadi se iz JWT
        );
    }

    private InterbankNegotiationEntity persistOutboundNegotiation(
            String localId,
            String remoteId,
            int lastByRouting,
            String lastById,
            boolean ongoing) {
        InterbankNegotiationEntity e = InterbankNegotiationEntity.builder()
                .id(localId)
                .buyerRoutingNumber(MY_ROUTING)
                .buyerId("C-" + MILE_ID)
                .sellerRoutingNumber(PARTNER_ROUTING)
                .sellerId("C-6")
                .stockTicker("AAPL")
                .amount(100)
                .priceCurrency("USD")
                .priceAmount(new BigDecimal("150.00"))
                .premiumCurrency("USD")
                .premiumAmount(new BigDecimal("5.00"))
                .settlementDate(OffsetDateTime.now().plusDays(30))
                .lastModifiedByRouting(lastByRouting)
                .lastModifiedById(lastById)
                .isOngoing(ongoing)
                .isAuthoritative(false)
                .remoteNegotiationId(remoteId)
                .build();
        return negRepo.save(e);
    }

    // ===== POST /api/interbank/otc/negotiations ==============================

    @Test
    void createOutboundHappyPath() throws Exception {
        ForeignBankId partnerId = new ForeignBankId(PARTNER_ROUTING, "neg-partner-001");
        when(interbankClient.outboundCreateNegotiation(eq(PARTNER_ROUTING), any(OtcOfferDto.class)))
                .thenReturn(partnerId);

        OutboundCreateNegotiationRequest req = sampleCreateRequest(PARTNER_ROUTING);

        mockMvc.perform(post("/api/interbank/otc/negotiations")
                        .with(mileJwt())
                        .contentType(MediaType.APPLICATION_JSON)
                        .content(mapper.writeValueAsString(req)))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.localId").exists())
                .andExpect(jsonPath("$.remoteForeignBankId.routingNumber").value(PARTNER_ROUTING))
                .andExpect(jsonPath("$.remoteForeignBankId.id").value("neg-partner-001"))
                .andExpect(jsonPath("$.state.amount").value(100))
                .andExpect(jsonPath("$.state.isOngoing").value(true));

        assertThat(negRepo.findAll()).hasSize(1);
        var saved = negRepo.findAll().get(0);
        assertThat(saved.getBuyerRoutingNumber()).isEqualTo(MY_ROUTING);
        assertThat(saved.getBuyerId()).isEqualTo("C-15");
        assertThat(saved.getSellerRoutingNumber()).isEqualTo(PARTNER_ROUTING);
        assertThat(saved.isAuthoritative()).isFalse();
        assertThat(saved.getRemoteNegotiationId()).isEqualTo("neg-partner-001");
        assertThat(saved.getLastModifiedByRouting()).isEqualTo(MY_ROUTING);
        verify(interbankClient, times(1)).outboundCreateNegotiation(
                eq(PARTNER_ROUTING), any(OtcOfferDto.class));
    }

    @Test
    void createOutboundRejectsSellerInOurBank() throws Exception {
        // sellerForeignBankId.routingNumber == MY_ROUTING → 400 (intra-bank, NE inter-bank)
        OutboundCreateNegotiationRequest req = sampleCreateRequest(MY_ROUTING);

        mockMvc.perform(post("/api/interbank/otc/negotiations")
                        .with(mileJwt())
                        .contentType(MediaType.APPLICATION_JSON)
                        .content(mapper.writeValueAsString(req)))
                .andExpect(status().isBadRequest())
                .andExpect(jsonPath("$.error").value(
                        org.hamcrest.Matchers.containsString("must NOT be my routing")));

        assertThat(negRepo.findAll()).isEmpty();
        verify(interbankClient, never()).outboundCreateNegotiation(anyInt(), any(OtcOfferDto.class));
    }

    @Test
    void createOutboundPastSettlementReturns400() throws Exception {
        OutboundCreateNegotiationRequest req = new OutboundCreateNegotiationRequest(
                "AAPL",
                OffsetDateTime.now().minusDays(1),  // past
                CurrencyCode.USD,
                new BigDecimal("150.00"),
                CurrencyCode.USD,
                new BigDecimal("5.00"),
                new ForeignBankId(PARTNER_ROUTING, "C-6"),
                100,
                null
        );

        mockMvc.perform(post("/api/interbank/otc/negotiations")
                        .with(mileJwt())
                        .contentType(MediaType.APPLICATION_JSON)
                        .content(mapper.writeValueAsString(req)))
                .andExpect(status().isBadRequest());
        verify(interbankClient, never()).outboundCreateNegotiation(anyInt(), any(OtcOfferDto.class));
    }

    // ===== GET /api/interbank/otc/negotiations (list) ========================

    @Test
    void listFiltersByPrincipal() throws Exception {
        persistOutboundNegotiation("neg-mine-1", "neg-partner-A", PARTNER_ROUTING, "C-6", true);
        // Drugog usera negotiation — ne sme da vrati nas listing
        InterbankNegotiationEntity other = InterbankNegotiationEntity.builder()
                .id("neg-other-1")
                .buyerRoutingNumber(MY_ROUTING)
                .buyerId("C-99")
                .sellerRoutingNumber(PARTNER_ROUTING)
                .sellerId("C-6")
                .stockTicker("MSFT")
                .amount(50)
                .priceCurrency("USD")
                .priceAmount(new BigDecimal("300.00"))
                .premiumCurrency("USD")
                .premiumAmount(new BigDecimal("8.00"))
                .settlementDate(OffsetDateTime.now().plusDays(20))
                .lastModifiedByRouting(MY_ROUTING)
                .lastModifiedById("C-99")
                .isOngoing(true)
                .isAuthoritative(false)
                .remoteNegotiationId("neg-partner-B")
                .build();
        negRepo.save(other);

        mockMvc.perform(get("/api/interbank/otc/negotiations").with(mileJwt()))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.length()").value(1))
                .andExpect(jsonPath("$[0].localId").value("neg-mine-1"));
    }

    // ===== GET /api/interbank/otc/negotiations/{id} ==========================

    @Test
    void getByLocalIdHappy() throws Exception {
        persistOutboundNegotiation("neg-local-1", "neg-partner-X", PARTNER_ROUTING, "C-6", true);

        mockMvc.perform(get("/api/interbank/otc/negotiations/{id}", "neg-local-1").with(mileJwt()))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.localId").value("neg-local-1"))
                .andExpect(jsonPath("$.remoteForeignBankId.routingNumber").value(PARTNER_ROUTING))
                .andExpect(jsonPath("$.remoteForeignBankId.id").value("neg-partner-X"));
    }

    @Test
    void getByRemoteIdAlsoWorks() throws Exception {
        persistOutboundNegotiation("neg-local-2", "neg-partner-Y", PARTNER_ROUTING, "C-6", true);

        mockMvc.perform(get("/api/interbank/otc/negotiations/{id}", "neg-partner-Y").with(mileJwt()))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.localId").value("neg-local-2"));
    }

    @Test
    void getNotFound() throws Exception {
        mockMvc.perform(get("/api/interbank/otc/negotiations/{id}", "does-not-exist").with(mileJwt()))
                .andExpect(status().isNotFound());
    }

    // ===== PUT /api/interbank/otc/negotiations/{id}/counter ==================

    @Test
    void counterOfferHappy204() throws Exception {
        // Partner zadnji modifikovao → na nas red
        persistOutboundNegotiation("neg-c-1", "neg-partner-Z", PARTNER_ROUTING, "C-6", true);
        when(interbankClient.outboundPutCounter(eq(PARTNER_ROUTING), any(ForeignBankId.class),
                any(OtcOfferDto.class)))
                .thenReturn(ResponseEntity.noContent().build());

        OutboundCounterOfferRequest req = new OutboundCounterOfferRequest(
                100,
                CurrencyCode.USD,
                new BigDecimal("145.00"),
                CurrencyCode.USD,
                new BigDecimal("4.50"),
                OffsetDateTime.now().plusDays(30)
        );

        mockMvc.perform(put("/api/interbank/otc/negotiations/{id}/counter", "neg-c-1")
                        .with(mileJwt())
                        .contentType(MediaType.APPLICATION_JSON)
                        .content(mapper.writeValueAsString(req)))
                .andExpect(status().isNoContent());

        var updated = negRepo.findById("neg-c-1").orElseThrow();
        assertThat(updated.getPriceAmount()).isEqualByComparingTo("145.00");
        assertThat(updated.getLastModifiedByRouting()).isEqualTo(MY_ROUTING);
        assertThat(updated.getLastModifiedById()).isEqualTo("C-15");
    }

    @Test
    void counterOfferTurnViolation409() throws Exception {
        // MI smo zadnji modifikovali — ne smemo opet
        persistOutboundNegotiation("neg-c-2", "neg-partner-W", MY_ROUTING, "C-15", true);

        OutboundCounterOfferRequest req = new OutboundCounterOfferRequest(
                100,
                CurrencyCode.USD,
                new BigDecimal("145.00"),
                CurrencyCode.USD,
                new BigDecimal("4.50"),
                OffsetDateTime.now().plusDays(30)
        );

        mockMvc.perform(put("/api/interbank/otc/negotiations/{id}/counter", "neg-c-2")
                        .with(mileJwt())
                        .contentType(MediaType.APPLICATION_JSON)
                        .content(mapper.writeValueAsString(req)))
                .andExpect(status().isConflict())
                .andExpect(jsonPath("$.error").value(org.hamcrest.Matchers.containsString("turn")));

        verify(interbankClient, never()).outboundPutCounter(anyInt(), any(ForeignBankId.class),
                any(OtcOfferDto.class));
    }

    @Test
    void counterOfferPartner409Propagated() throws Exception {
        // Partner odbije sa 409 (npr. neki state mismatch) — propagate
        persistOutboundNegotiation("neg-c-3", "neg-partner-V", PARTNER_ROUTING, "C-6", true);
        when(interbankClient.outboundPutCounter(eq(PARTNER_ROUTING), any(ForeignBankId.class),
                any(OtcOfferDto.class)))
                .thenReturn(ResponseEntity.status(409).build());

        OutboundCounterOfferRequest req = new OutboundCounterOfferRequest(
                100,
                CurrencyCode.USD,
                new BigDecimal("145.00"),
                CurrencyCode.USD,
                new BigDecimal("4.50"),
                OffsetDateTime.now().plusDays(30)
        );

        mockMvc.perform(put("/api/interbank/otc/negotiations/{id}/counter", "neg-c-3")
                        .with(mileJwt())
                        .contentType(MediaType.APPLICATION_JSON)
                        .content(mapper.writeValueAsString(req)))
                .andExpect(status().isConflict());

        // Local mirror NE sme biti update-ovan
        var unchanged = negRepo.findById("neg-c-3").orElseThrow();
        assertThat(unchanged.getPriceAmount()).isEqualByComparingTo("150.00");
        assertThat(unchanged.getLastModifiedByRouting()).isEqualTo(PARTNER_ROUTING);
    }

    // ===== POST /api/interbank/otc/negotiations/{id}/accept ==================

    @Test
    void acceptHappy204() throws Exception {
        // Partner (seller) zadnji modifikovao → mi (buyer) accept-ujemo
        persistOutboundNegotiation("neg-a-1", "neg-partner-U", PARTNER_ROUTING, "C-6", true);
        when(interbankClient.outboundAccept(eq(PARTNER_ROUTING), any(ForeignBankId.class)))
                .thenReturn(ResponseEntity.noContent().build());

        mockMvc.perform(post("/api/interbank/otc/negotiations/{id}/accept", "neg-a-1")
                        .with(mileJwt()))
                .andExpect(status().isNoContent());

        verify(interbankClient, times(1)).outboundAccept(eq(PARTNER_ROUTING), any(ForeignBankId.class));
    }

    @Test
    void acceptTurnViolation409() throws Exception {
        // MI smo zadnji modifikovali — partner sledeci treba da odgovori
        persistOutboundNegotiation("neg-a-2", "neg-partner-T", MY_ROUTING, "C-15", true);

        mockMvc.perform(post("/api/interbank/otc/negotiations/{id}/accept", "neg-a-2")
                        .with(mileJwt()))
                .andExpect(status().isConflict());

        verify(interbankClient, never()).outboundAccept(anyInt(), any(ForeignBankId.class));
    }

    // ===== DELETE /api/interbank/otc/negotiations/{id} =======================

    @Test
    void deleteHappy204() throws Exception {
        persistOutboundNegotiation("neg-d-1", "neg-partner-S", PARTNER_ROUTING, "C-6", true);
        when(interbankClient.outboundDelete(eq(PARTNER_ROUTING), any(ForeignBankId.class)))
                .thenReturn(ResponseEntity.noContent().build());

        mockMvc.perform(delete("/api/interbank/otc/negotiations/{id}", "neg-d-1").with(mileJwt()))
                .andExpect(status().isNoContent());

        var closed = negRepo.findById("neg-d-1").orElseThrow();
        assertThat(closed.isOngoing()).isFalse();
    }

    @Test
    void unauthenticatedReturns401() throws Exception {
        // Bez JWT — security-lib JWT chain rejects sa 401
        mockMvc.perform(get("/api/interbank/otc/negotiations"))
                .andExpect(status().isUnauthorized());
    }
}
