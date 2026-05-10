package com.banka1.banking_service.card_service.service.implementation;

import com.banka1.banking_service.card_service.domain.AuthorizedPerson;
import com.banka1.banking_service.card_service.domain.Card;
import com.banka1.banking_service.card_service.domain.enums.AuthorizedPersonGender;
import com.banka1.banking_service.card_service.domain.enums.CardBrand;
import com.banka1.banking_service.card_service.domain.enums.CardRequestRecipientType;
import com.banka1.banking_service.card_service.domain.enums.CardStatus;
import com.banka1.banking_service.card_service.dto.card_creation.internal.CardCreationResult;
import com.banka1.banking_service.card_service.dto.card_creation.internal.CreateCardCommand;
import com.banka1.banking_service.card_service.dto.card_creation.request.AuthorizedPersonRequestDto;
import com.banka1.banking_service.card_service.dto.card_creation.request.AutoCardCreationRequestDto;
import com.banka1.banking_service.card_service.dto.card_creation.request.BusinessCardRequestDto;
import com.banka1.banking_service.card_service.dto.card_creation.request.ClientCardRequestDto;
import com.banka1.banking_service.card_service.mapper.CardCreationResponseMapper;
import com.banka1.banking_service.card_service.rabbitMQ.RabbitClient;
import com.banka1.banking_service.card_service.repository.AuthorizedPersonRepository;
import com.banka1.banking_service.card_service.repository.CardRepository;
import com.banka1.banking_service.card_service.rest_client.*;
import com.banka1.banking_service.card_service.service.CardCreationService;
import com.banka1.banking_service.card_service.domain.enums.AccountOwnershipType;
import com.banka1.banking_service.card_service.exception.BusinessException;
import com.banka1.banking_service.card_service.exception.ErrorCode;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.ArgumentCaptor;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;
import org.springframework.test.util.ReflectionTestUtils;
import org.springframework.transaction.support.TransactionSynchronization;
import org.springframework.transaction.support.TransactionSynchronizationManager;

import java.math.BigDecimal;
import java.time.LocalDate;
import java.util.Arrays;
import java.util.Optional;

import static org.junit.jupiter.api.Assertions.*;
import static org.mockito.ArgumentMatchers.*;
import static org.mockito.Mockito.*;

@ExtendWith(MockitoExtension.class)
class CardRequestServiceImplTest {

    @Mock
    private CardCreationService cardCreationService;

    @Mock
    private CardRepository cardRepository;

    @Mock
    private AuthorizedPersonRepository authorizedPersonRepository;

    @Mock
    private ClientService clientService;

    @Mock
    private VerificationService verificationService;

    @Mock
    private RabbitClient rabbitClient;

    private CardRequestServiceImpl service;

    @BeforeEach
    void setUp() {
        TransactionSynchronizationManager.initSynchronization();
        service = new CardRequestServiceImpl(
                cardCreationService,
                cardRepository,
                authorizedPersonRepository,
                clientService,
                verificationService,
                rabbitClient,
                new CardCreationResponseMapper()
        );
        ReflectionTestUtils.setField(service, "automaticCardDefaultLimit", BigDecimal.valueOf(1_000_000));
    }

    @AfterEach
    void tearDown() {
        if (TransactionSynchronizationManager.isSynchronizationActive()) {
            TransactionSynchronizationManager.clearSynchronization();
        }
    }

    @Test
    void handleClientRequestUsesVerifiedExternalSessionCreatesCardAndSchedulesSuccessEmail() {
        ClientCardRequestDto request = new ClientCardRequestDto();
        request.setAccountNumber("265000000000123456");
        request.setCardBrand(CardBrand.VISA);
        request.setCardLimit(BigDecimal.valueOf(2500));
        request.setVerificationId(77L);

        Card createdCard = card(501L, 1L, null);

        when(verificationService.getStatus(77L))
                .thenReturn(new VerificationStatusResponse(77L, VerificationStatus.VERIFIED));
        when(cardRepository.countByAccountNumberAndClientIdAndAuthorizedPersonIdIsNullAndStatusNot(
                "265000000000123456", 1L, CardStatus.DEACTIVATED
        )).thenReturn(1L);
        when(cardCreationService.createCard(any(CreateCardCommand.class)))
                .thenReturn(new CardCreationResult(createdCard, "555"));
        when(clientService.getNotificationRecipient(1L)).thenReturn(ownerRecipient());

        var response = service.processManualCardRequest(
                new AccountNotificationContextDto(AccountOwnershipType.PERSONAL, 1L),
                request
        );

        assertEquals("COMPLETED", response.status());
        assertEquals(501L, response.createdCard().cardId());
        assertEquals("4111111111111111", response.createdCard().cardNumber());
        assertEquals("555", response.createdCard().plainCvv());
        verify(rabbitClient, never()).sendCardNotification(any(), any());

        TransactionSynchronizationManager.getSynchronizations().forEach(TransactionSynchronization::afterCommit);

        verify(rabbitClient, times(1)).sendCardNotification(
                eq(com.banka1.banking_service.card_service.dto.enums.CardNotificationType.CARD_REQUEST_SUCCESS),
                any()
        );
    }

    @Test
    void handleClientRequestRejectsNonVerifiedExternalSession() {
        ClientCardRequestDto request = new ClientCardRequestDto();
        request.setAccountNumber("265000000000123456");
        request.setCardBrand(CardBrand.VISA);
        request.setCardLimit(BigDecimal.valueOf(1200));
        request.setVerificationId(10L);

        when(verificationService.getStatus(10L))
                .thenReturn(new VerificationStatusResponse(10L, VerificationStatus.PENDING));

        BusinessException exception = assertThrows(
                BusinessException.class,
                () -> service.processManualCardRequest(
                        new AccountNotificationContextDto(AccountOwnershipType.PERSONAL, 1L),
                        request
                )
        );

        assertEquals(ErrorCode.INVALID_REQUEST_STATE, exception.getErrorCode());
        verifyNoMoreInteractions(cardCreationService, clientService, rabbitClient);
    }

    @Test
    void handleClientRequestUsesOwnerFromAccountContextForCardCreationAndNotification() {
        ClientCardRequestDto request = new ClientCardRequestDto();
        request.setAccountNumber("265000000000123456");
        request.setCardBrand(CardBrand.VISA);
        request.setCardLimit(BigDecimal.valueOf(1200));
        request.setVerificationId(88L);

        Card createdCard = card(600L, 55L, null);

        when(verificationService.getStatus(88L))
                .thenReturn(new VerificationStatusResponse(88L, VerificationStatus.VERIFIED));
        when(cardRepository.countByAccountNumberAndClientIdAndAuthorizedPersonIdIsNullAndStatusNot(
                "265000000000123456", 55L, CardStatus.DEACTIVATED
        )).thenReturn(0L);
        when(cardCreationService.createCard(any(CreateCardCommand.class)))
                .thenReturn(new CardCreationResult(createdCard, "444"));
        when(clientService.getNotificationRecipient(55L))
                .thenReturn(new ClientNotificationRecipientDto(55L, "Owner", "User", "owner@example.com"));

        service.processManualCardRequest(new AccountNotificationContextDto(AccountOwnershipType.PERSONAL, 55L), request);

        verify(cardCreationService).createCard(argThat(command ->
                command != null
                        && Long.valueOf(55L).equals(command.clientId())
                        && "265000000000123456".equals(command.accountNumber())
        ));
        verify(clientService).getNotificationRecipient(55L);
    }

    @Test
    void handleBusinessRequestCreatesAuthorizedPersonCardAndSchedulesTwoSuccessEmails() {
        BusinessCardRequestDto request = new BusinessCardRequestDto();
        request.setAccountNumber("265000000000999999");
        request.setRecipientType(CardRequestRecipientType.AUTHORIZED_PERSON);
        request.setCardBrand(CardBrand.MASTERCARD);
        request.setCardLimit(BigDecimal.valueOf(900));
        request.setVerificationId(15L);
        request.setAuthorizedPerson(authorizedPersonRequest());

        Card createdCard = card(777L, 1L, 7L);

        when(verificationService.getStatus(15L))
                .thenReturn(new VerificationStatusResponse(15L, VerificationStatus.VERIFIED));
        when(authorizedPersonRepository.findByEmailIgnoreCaseAndFirstNameIgnoreCaseAndLastNameIgnoreCaseAndDateOfBirth(
                "ana@example.com",
                "Ana",
                "Anic",
                LocalDate.of(1994, 2, 10)
        )).thenReturn(Optional.empty());
        when(cardRepository.countByAccountNumberAndAuthorizedPersonIdAndStatusNot(
                "265000000000999999", 7L, CardStatus.DEACTIVATED
        )).thenReturn(0L);
        when(cardCreationService.createCard(any(CreateCardCommand.class)))
                .thenReturn(new CardCreationResult(createdCard, "123"));
        when(authorizedPersonRepository.save(any(AuthorizedPerson.class))).thenAnswer(invocation -> {
            AuthorizedPerson authorizedPerson = invocation.getArgument(0);
            if (authorizedPerson.getId() == null) {
                authorizedPerson.setId(7L);
            }
            return authorizedPerson;
        });
        when(clientService.getNotificationRecipient(1L)).thenReturn(ownerRecipient());

        var response = service.processBusinessCardRequest(
                new AccountNotificationContextDto(AccountOwnershipType.BUSINESS, 1L),
                request
        );

        assertEquals("COMPLETED", response.status());

        ArgumentCaptor<AuthorizedPerson> captor = ArgumentCaptor.forClass(AuthorizedPerson.class);
        verify(authorizedPersonRepository, times(2)).save(captor.capture());
        AuthorizedPerson persistedAuthorizedPerson = captor.getAllValues().getLast();
        assertEquals(1, persistedAuthorizedPerson.getCardIds().size());
        assertEquals(777L, persistedAuthorizedPerson.getCardIds().getFirst());

        TransactionSynchronizationManager.getSynchronizations().forEach(TransactionSynchronization::afterCommit);

        verify(rabbitClient, times(2)).sendCardNotification(
                eq(com.banka1.banking_service.card_service.dto.enums.CardNotificationType.CARD_REQUEST_SUCCESS),
                any()
        );
    }

    @Test
    void handleBusinessRequestReusesExistingAuthorizedPersonWhenIdentityMatches() {
        BusinessCardRequestDto request = new BusinessCardRequestDto();
        request.setAccountNumber("265000000000999999");
        request.setRecipientType(CardRequestRecipientType.AUTHORIZED_PERSON);
        request.setCardBrand(CardBrand.VISA);
        request.setCardLimit(BigDecimal.valueOf(300));
        request.setVerificationId(33L);
        request.setAuthorizedPerson(authorizedPersonRequest());

        AuthorizedPerson existing = new AuthorizedPerson();
        existing.setId(20L);
        existing.setFirstName("Ana");
        existing.setLastName("Anic");
        existing.setEmail("ana@example.com");
        existing.setDateOfBirth(LocalDate.of(1994, 2, 10));

        Card createdCard = card(880L, 1L, 20L);

        when(verificationService.getStatus(33L))
                .thenReturn(new VerificationStatusResponse(33L, VerificationStatus.VERIFIED));
        when(authorizedPersonRepository.findByEmailIgnoreCaseAndFirstNameIgnoreCaseAndLastNameIgnoreCaseAndDateOfBirth(
                "ana@example.com",
                "Ana",
                "Anic",
                LocalDate.of(1994, 2, 10)
        )).thenReturn(Optional.of(existing));
        when(cardRepository.countByAccountNumberAndAuthorizedPersonIdAndStatusNot(
                "265000000000999999", 20L, CardStatus.DEACTIVATED
        )).thenReturn(0L);
        when(cardCreationService.createCard(any(CreateCardCommand.class)))
                .thenReturn(new CardCreationResult(createdCard, "456"));
        when(authorizedPersonRepository.save(any(AuthorizedPerson.class))).thenAnswer(invocation -> invocation.getArgument(0));
        when(clientService.getNotificationRecipient(1L)).thenReturn(ownerRecipient());

        var response = service.processBusinessCardRequest(
                new AccountNotificationContextDto(AccountOwnershipType.BUSINESS, 1L),
                request
        );

        assertEquals("COMPLETED", response.status());
        ArgumentCaptor<CreateCardCommand> commandCaptor = ArgumentCaptor.forClass(CreateCardCommand.class);
        verify(cardCreationService).createCard(commandCaptor.capture());
        assertEquals(20L, commandCaptor.getValue().authorizedPersonId());
    }

    @Test
    void createAutomaticCardUsesConfiguredDefaultLimitAndRandomSupportedBrand() {
        AutoCardCreationRequestDto request = new AutoCardCreationRequestDto();
        request.setAccountNumber("265000000000123456");
        request.setClientId(1L);

        Card createdCard = card(900L, 1L, null);
        createdCard.setCardLimit(BigDecimal.valueOf(1_000_000));

        when(cardCreationService.createCard(any(CreateCardCommand.class)))
                .thenReturn(new CardCreationResult(createdCard, "321"));

        service.createAutomaticCard(request);

        ArgumentCaptor<CreateCardCommand> captor = ArgumentCaptor.forClass(CreateCardCommand.class);
        verify(cardCreationService).createCard(captor.capture());
        assertTrue(Arrays.asList(CardBrand.values()).contains(captor.getValue().cardBrand()));
        assertEquals(BigDecimal.valueOf(1_000_000), captor.getValue().cardLimit());
    }

    private AuthorizedPersonRequestDto authorizedPersonRequest() {
        AuthorizedPersonRequestDto dto = new AuthorizedPersonRequestDto();
        dto.setFirstName("Ana");
        dto.setLastName("Anic");
        dto.setDateOfBirth(LocalDate.of(1994, 2, 10));
        dto.setGender(AuthorizedPersonGender.FEMALE);
        dto.setEmail("ana@example.com");
        dto.setPhone("0601234567");
        dto.setAddress("Adresa 1");
        return dto;
    }

    private ClientNotificationRecipientDto ownerRecipient() {
        return new ClientNotificationRecipientDto(1L, "Pera", "Peric", "pera@example.com");
    }

    private Card card(Long id, Long clientId, Long authorizedPersonId) {
        Card card = new Card();
        card.setId(id);
        card.setCardNumber("4111111111111111");
        card.setAccountNumber("265000000000123456");
        card.setClientId(clientId);
        card.setAuthorizedPersonId(authorizedPersonId);
        card.setCardName("Visa Debit");
        card.setCreationDate(LocalDate.of(2026, 3, 25));
        card.setExpirationDate(LocalDate.of(2031, 3, 25));
        card.setStatus(CardStatus.ACTIVE);
        card.setCardLimit(BigDecimal.valueOf(1200));
        return card;
    }
}
