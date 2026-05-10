package com.banka1.banking_service.card_service.integration;

import com.banka1.banking_service.card_service.domain.AuthorizedPerson;
import com.banka1.banking_service.card_service.domain.Card;
import com.banka1.banking_service.card_service.domain.enums.CardStatus;
import com.banka1.banking_service.card_service.dto.card_management.internal.CardNotificationDto;
import com.banka1.banking_service.card_service.dto.enums.CardNotificationType;
import com.banka1.banking_service.card_service.rabbitMQ.RabbitClient;
import com.banka1.banking_service.card_service.repository.AuthorizedPersonRepository;
import com.banka1.banking_service.card_service.repository.CardRepository;
import com.banka1.banking_service.card_service.rest_client.*;
import com.banka1.banking_service.account_service.service.AccountService;
import com.banka1.banking_service.account_service.dto.response.InternalAccountDetailsDto;
import com.banka1.banking_service.card_service.domain.enums.AccountOwnershipType;
import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.boot.test.context.SpringBootTest;
import org.springframework.boot.webmvc.test.autoconfigure.AutoConfigureMockMvc;
import org.springframework.http.MediaType;
import org.springframework.security.core.authority.SimpleGrantedAuthority;
import org.springframework.test.context.ActiveProfiles;
import org.springframework.test.context.bean.override.mockito.MockitoBean;
import org.springframework.test.web.servlet.MockMvc;
import org.springframework.test.web.servlet.request.RequestPostProcessor;

import java.math.BigDecimal;
import java.time.LocalDate;
import java.time.temporal.ChronoUnit;
import java.util.List;

import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.ArgumentMatchers.eq;
import static org.mockito.Mockito.*;
import static org.springframework.security.test.web.servlet.request.SecurityMockMvcRequestPostProcessors.jwt;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.post;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.jsonPath;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.status;

@SpringBootTest
@AutoConfigureMockMvc
@ActiveProfiles("test")
class CardCreationControllerIntegrationTest {

    private static final Long OWNER_CLIENT_ID = 1L;
    private static final String PERSONAL_ACCOUNT_NUMBER = "265000000000123456";
    private static final String BUSINESS_ACCOUNT_NUMBER = "265000000000999999";

    @Autowired
    private MockMvc mockMvc;

    private final ObjectMapper objectMapper = new ObjectMapper().findAndRegisterModules();

    @Autowired
    private CardRepository cardRepository;

    @Autowired
    private AuthorizedPersonRepository authorizedPersonRepository;

    @Value("${card.creation.automatic.default-limit}")
    private BigDecimal automaticDefaultLimit;

    @MockitoBean
    private AccountService accountService;

    @MockitoBean
    private ClientService clientService;

    @MockitoBean
    private VerificationService verificationService;

    @MockitoBean
    private RabbitClient rabbitClient;

    @BeforeEach
    void setUp() {
        cardRepository.deleteAll();
        authorizedPersonRepository.deleteAll();
        reset(accountService, clientService, verificationService, rabbitClient);
    }

    @Test
    @DisplayName("POST /auto persists a real card and returns the one-time creation payload")
    void autoCreateCard_persistsCardAndReturnsCreationPayload() throws Exception {
        String requestBody = """
                {
                  "clientId": 1,
                  "accountNumber": "265000000000123456"
                }
                """;

        String responseBody = mockMvc.perform(post("/auto")
                        .with(serviceJwt())
                .contentType(MediaType.APPLICATION_JSON)
                .content(requestBody))
                .andExpect(status().isCreated())
                .andExpect(jsonPath("$.cardId").exists())
                .andExpect(jsonPath("$.cardNumber").exists())
                .andExpect(jsonPath("$.plainCvv").exists())
                .andExpect(jsonPath("$.expirationDate").exists())
                .andExpect(jsonPath("$.cardName").isNotEmpty())
                .andReturn()
                .getResponse()
                .getContentAsString();

        JsonNode responseJson = objectMapper.readTree(responseBody);
        List<Card> persistedCards = cardRepository.findByClientId(OWNER_CLIENT_ID);

        assertThat(persistedCards).hasSize(1);

        Card persistedCard = persistedCards.get(0);
        assertThat(persistedCard.getAccountNumber()).isEqualTo(PERSONAL_ACCOUNT_NUMBER);
        assertThat(persistedCard.getClientId()).isEqualTo(OWNER_CLIENT_ID);
        assertThat(persistedCard.getAuthorizedPersonId()).isNull();
        assertThat(persistedCard.getCardLimit()).isEqualByComparingTo(automaticDefaultLimit);
        assertThat(persistedCard.getStatus()).isEqualTo(CardStatus.ACTIVE);
        assertThat(ChronoUnit.YEARS.between(persistedCard.getCreationDate(), persistedCard.getExpirationDate())).isEqualTo(5);
        assertThat(responseJson.get("cardId").asLong()).isEqualTo(persistedCard.getId());
        assertThat(responseJson.get("cardNumber").asText()).isEqualTo(persistedCard.getCardNumber());
        assertThat(responseJson.get("cardName").asText()).isEqualTo(persistedCard.getCardName());
        assertThat(responseJson.get("plainCvv").asText()).matches("\\d{3}");
        assertThat(persistedCard.getCvv()).isNotEqualTo(responseJson.get("plainCvv").asText());
    }

    @Test
    @DisplayName("POST /request creates a personal card when verification-service returns VERIFIED")
    void manualPersonalCardRequest_createsCardAfterExternalVerification() throws Exception {
        when(accountService.getAccountDetails(PERSONAL_ACCOUNT_NUMBER))
                .thenReturn(accountDetails(AccountOwnershipType.PERSONAL, OWNER_CLIENT_ID));
        when(verificationService.getStatus(77L))
                .thenReturn(new VerificationStatusResponse(77L, VerificationStatus.VERIFIED));
        when(clientService.getNotificationRecipient(OWNER_CLIENT_ID))
                .thenReturn(ownerRecipient());

        String requestPayload = """
                {
                  "accountNumber": "265000000000123456",
                  "cardBrand": "VISA",
                  "cardLimit": 1500.00,
                  "verificationId": 77
                }
                """;

        String responseBody = mockMvc.perform(post("/request")
                        .with(clientJwt(OWNER_CLIENT_ID))
                        .contentType(MediaType.APPLICATION_JSON)
                        .content(requestPayload))
                .andExpect(status().isCreated())
                .andExpect(jsonPath("$.status").value("COMPLETED"))
                .andExpect(jsonPath("$.message").value("Card created successfully."))
                .andExpect(jsonPath("$.createdCard.cardId").exists())
                .andExpect(jsonPath("$.createdCard.cardNumber").exists())
                .andExpect(jsonPath("$.createdCard.plainCvv").exists())
                .andReturn()
                .getResponse()
                .getContentAsString();

        JsonNode responseJson = objectMapper.readTree(responseBody);
        List<Card> persistedCards = cardRepository.findByClientId(OWNER_CLIENT_ID);

        assertThat(persistedCards).hasSize(1);
        Card persistedCard = persistedCards.get(0);
        assertThat(persistedCard.getAccountNumber()).isEqualTo(PERSONAL_ACCOUNT_NUMBER);
        assertThat(persistedCard.getAuthorizedPersonId()).isNull();
        assertThat(persistedCard.getCardLimit()).isEqualByComparingTo("1500.00");
        assertThat(responseJson.get("createdCard").get("cardId").asLong()).isEqualTo(persistedCard.getId());
        assertThat(responseJson.get("createdCard").get("cardNumber").asText()).isEqualTo(persistedCard.getCardNumber());

        verify(rabbitClient, times(1)).sendCardNotification(eq(CardNotificationType.CARD_REQUEST_SUCCESS), eqNotificationForOwner());
    }

    @Test
    @DisplayName("POST /request allows ADMIN callers while still assigning the card to the real account owner")
    void manualPersonalCardRequest_byAdminAssignsCardToAccountOwner() throws Exception {
        when(accountService.getAccountDetails(PERSONAL_ACCOUNT_NUMBER))
                .thenReturn(accountDetails(AccountOwnershipType.PERSONAL, OWNER_CLIENT_ID));
        when(verificationService.getStatus(77L))
                .thenReturn(new VerificationStatusResponse(77L, VerificationStatus.VERIFIED));
        when(clientService.getNotificationRecipient(OWNER_CLIENT_ID))
                .thenReturn(ownerRecipient());

        String requestPayload = """
                {
                  "accountNumber": "265000000000123456",
                  "cardBrand": "VISA",
                  "cardLimit": 1500.00,
                  "verificationId": 77
                }
                """;

        String responseBody = mockMvc.perform(post("/request")
                        .with(adminJwt(999L))
                .contentType(MediaType.APPLICATION_JSON)
                .content(requestPayload))
                .andExpect(status().isCreated())
                .andExpect(jsonPath("$.status").value("COMPLETED"))
                .andExpect(jsonPath("$.createdCard.cardId").exists())
                .andExpect(jsonPath("$.createdCard.cardNumber").exists())
                .andReturn()
                .getResponse()
                .getContentAsString();

        JsonNode responseJson = objectMapper.readTree(responseBody);
        List<Card> persistedCards = cardRepository.findByClientId(OWNER_CLIENT_ID);

        assertThat(persistedCards).hasSize(1);
        Card persistedCard = persistedCards.get(0);
        assertThat(persistedCard.getClientId()).isEqualTo(OWNER_CLIENT_ID);
        assertThat(responseJson.get("createdCard").get("cardId").asLong()).isEqualTo(persistedCard.getId());
        assertThat(responseJson.get("createdCard").get("cardNumber").asText()).isEqualTo(persistedCard.getCardNumber());

        verify(rabbitClient, times(1)).sendCardNotification(eq(CardNotificationType.CARD_REQUEST_SUCCESS), eqNotificationForOwner());
    }

    @Test
    @DisplayName("POST /request/business creates a business-owner card when verification-service returns VERIFIED")
    void businessOwnerRequest_createsOwnerCardAfterExternalVerification() throws Exception {
        when(accountService.getAccountDetails(BUSINESS_ACCOUNT_NUMBER))
                .thenReturn(accountDetails(AccountOwnershipType.BUSINESS, OWNER_CLIENT_ID));
        when(verificationService.getStatus(88L))
                .thenReturn(new VerificationStatusResponse(88L, VerificationStatus.VERIFIED));
        when(clientService.getNotificationRecipient(OWNER_CLIENT_ID))
                .thenReturn(ownerRecipient());

        String requestPayload = """
                {
                  "accountNumber": "265000000000999999",
                  "recipientType": "OWNER",
                  "cardBrand": "DINACARD",
                  "cardLimit": 2500.00,
                  "verificationId": 88
                }
                """;

        String responseBody = mockMvc.perform(post("/request/business")
                        .with(clientJwt(OWNER_CLIENT_ID))
                .contentType(MediaType.APPLICATION_JSON)
                .content(requestPayload))
                .andExpect(status().isCreated())
                .andExpect(jsonPath("$.status").value("COMPLETED"))
                .andExpect(jsonPath("$.createdCard.cardId").exists())
                .andExpect(jsonPath("$.createdCard.cardNumber").exists())
                .andReturn()
                .getResponse()
                .getContentAsString();

        JsonNode responseJson = objectMapper.readTree(responseBody);
        List<Card> persistedCards = cardRepository.findByClientId(OWNER_CLIENT_ID);

        assertThat(authorizedPersonRepository.count()).isZero();
        assertThat(persistedCards).hasSize(1);

        Card persistedCard = persistedCards.get(0);
        assertThat(persistedCard.getAuthorizedPersonId()).isNull();
        assertThat(persistedCard.getCardLimit()).isEqualByComparingTo("2500.00");
        assertThat(responseJson.get("createdCard").get("cardId").asLong()).isEqualTo(persistedCard.getId());
        assertThat(responseJson.get("createdCard").get("cardNumber").asText()).isEqualTo(persistedCard.getCardNumber());

        verify(rabbitClient, times(1)).sendCardNotification(eq(CardNotificationType.CARD_REQUEST_SUCCESS), eqNotificationForOwner());
    }

    @Test
    @DisplayName("POST /request/business creates an authorized person and links the new business card to them")
    void businessAuthorizedPersonRequest_materializesAuthorizedPersonAndPersistsLinkedCard() throws Exception {
        when(accountService.getAccountDetails(BUSINESS_ACCOUNT_NUMBER))
                .thenReturn(accountDetails(AccountOwnershipType.BUSINESS, OWNER_CLIENT_ID));
        when(verificationService.getStatus(99L))
                .thenReturn(new VerificationStatusResponse(99L, VerificationStatus.VERIFIED));
        when(clientService.getNotificationRecipient(OWNER_CLIENT_ID))
                .thenReturn(ownerRecipient());

        String requestPayload = """
                {
                  "accountNumber": "265000000000999999",
                  "recipientType": "AUTHORIZED_PERSON",
                  "cardBrand": "MASTERCARD",
                  "cardLimit": 800.00,
                  "verificationId": 99,
                  "authorizedPerson": {
                    "firstName": "Ana",
                    "lastName": "Anic",
                    "dateOfBirth": "1994-02-10",
                    "gender": "FEMALE",
                    "email": "ana@example.com",
                    "phone": "0601234567",
                    "address": "Adresa 1"
                  }
                }
                """;

        String responseBody = mockMvc.perform(post("/request/business")
                        .with(clientJwt(OWNER_CLIENT_ID))
                .contentType(MediaType.APPLICATION_JSON)
                .content(requestPayload))
                .andExpect(status().isCreated())
                .andExpect(jsonPath("$.status").value("COMPLETED"))
                .andExpect(jsonPath("$.createdCard.cardId").exists())
                .andExpect(jsonPath("$.createdCard.cardNumber").exists())
                .andReturn()
                .getResponse()
                .getContentAsString();

        JsonNode responseJson = objectMapper.readTree(responseBody);
        List<Card> persistedCards = cardRepository.findByClientId(OWNER_CLIENT_ID);
        List<AuthorizedPerson> authorizedPeople = authorizedPersonRepository.findAll();

        assertThat(authorizedPeople).hasSize(1);
        assertThat(persistedCards).hasSize(1);

        AuthorizedPerson authorizedPerson = authorizedPeople.get(0);
        Card persistedCard = persistedCards.get(0);

        assertThat(authorizedPerson.getFirstName()).isEqualTo("Ana");
        assertThat(authorizedPerson.getLastName()).isEqualTo("Anic");
        assertThat(authorizedPerson.getDateOfBirth()).isEqualTo(LocalDate.of(1994, 2, 10));
        assertThat(authorizedPerson.getEmail()).isEqualTo("ana@example.com");
        assertThat(persistedCard.getAuthorizedPersonId()).isEqualTo(authorizedPerson.getId());
        assertThat(authorizedPerson.getCardIds()).containsExactly(persistedCard.getId());
        assertThat(responseJson.get("createdCard").get("cardId").asLong()).isEqualTo(persistedCard.getId());
        assertThat(responseJson.get("createdCard").get("cardNumber").asText()).isEqualTo(persistedCard.getCardNumber());

        var notificationCaptor = org.mockito.ArgumentCaptor.forClass(CardNotificationDto.class);
        verify(rabbitClient, times(2)).sendCardNotification(eq(CardNotificationType.CARD_REQUEST_SUCCESS), notificationCaptor.capture());

        assertThat(notificationCaptor.getAllValues())
                .extracting(CardNotificationDto::getUserEmail)
                .containsExactlyInAnyOrder("pera@example.com", "ana@example.com");
    }

    private CardNotificationDto eqNotificationForOwner() {
        return org.mockito.ArgumentMatchers.argThat(notification ->
                notification != null
                        && "pera@example.com".equals(notification.getUserEmail())
                        && notification.getTemplateVariables() != null
                        && notification.getTemplateVariables().containsKey("cardNumber")
        );
    }

    private ClientNotificationRecipientDto ownerRecipient() {
        return new ClientNotificationRecipientDto(OWNER_CLIENT_ID, "Pera", "Peric", "pera@example.com");
    }

    private RequestPostProcessor clientJwt(Long clientId) {
        return jwt().authorities(new SimpleGrantedAuthority("ROLE_CLIENT_BASIC"))
                .jwt(token -> token.claim("id", clientId));
    }

    private RequestPostProcessor serviceJwt() {
        return jwt().authorities(new SimpleGrantedAuthority("ROLE_SERVICE"))
                .jwt(token -> token.claim("id", 999L));
    }

    private RequestPostProcessor adminJwt(Long adminId) {
        return jwt().authorities(new SimpleGrantedAuthority("ROLE_ADMIN"))
                .jwt(token -> token.claim("id", adminId));
    }
    private InternalAccountDetailsDto accountDetails(AccountOwnershipType type, Long ownerId) {
        return new InternalAccountDetailsDto(null, null, ownerId, "RSD", null, "ACTIVE", type == null ? null : type.name(), null, null);
    }
}