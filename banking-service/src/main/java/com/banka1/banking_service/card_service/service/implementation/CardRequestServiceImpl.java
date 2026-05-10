package com.banka1.banking_service.card_service.service.implementation;

import com.banka1.banking_service.card_service.domain.AuthorizedPerson;
import com.banka1.banking_service.card_service.domain.Card;
import com.banka1.banking_service.card_service.domain.enums.CardBrand;
import com.banka1.banking_service.card_service.domain.enums.CardRequestRecipientType;
import com.banka1.banking_service.card_service.domain.enums.CardStatus;
import com.banka1.banking_service.card_service.dto.card_creation.internal.CardCreationResult;
import com.banka1.banking_service.card_service.dto.card_creation.internal.CreateCardCommand;
import com.banka1.banking_service.card_service.dto.card_creation.request.AuthorizedPersonRequestDto;
import com.banka1.banking_service.card_service.dto.card_creation.request.AutoCardCreationRequestDto;
import com.banka1.banking_service.card_service.dto.card_creation.request.BusinessCardRequestDto;
import com.banka1.banking_service.card_service.dto.card_creation.request.ClientCardRequestDto;
import com.banka1.banking_service.card_service.dto.card_creation.response.CardCreationResponseDto;
import com.banka1.banking_service.card_service.dto.card_creation.response.CardRequestResponseDto;
import com.banka1.banking_service.card_service.dto.card_management.internal.CardNotificationDto;
import com.banka1.banking_service.card_service.dto.enums.CardNotificationType;
import com.banka1.banking_service.card_service.exception.BusinessException;
import com.banka1.banking_service.card_service.exception.ErrorCode;
import com.banka1.banking_service.card_service.mapper.CardCreationResponseMapper;
import com.banka1.banking_service.card_service.rabbitMQ.RabbitClient;
import com.banka1.banking_service.card_service.repository.AuthorizedPersonRepository;
import com.banka1.banking_service.card_service.repository.CardRepository;
import com.banka1.banking_service.card_service.rest_client.AccountNotificationContextDto;
import com.banka1.banking_service.card_service.rest_client.ClientNotificationRecipientDto;
import com.banka1.banking_service.card_service.rest_client.ClientService;
import com.banka1.banking_service.card_service.rest_client.VerificationService;
import com.banka1.banking_service.card_service.rest_client.VerificationStatus;
import com.banka1.banking_service.card_service.service.CardCreationService;
import com.banka1.banking_service.card_service.service.CardRequestService;
import com.banka1.banking_service.card_service.util.SensitiveDataMasker;
import lombok.RequiredArgsConstructor;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;
import org.springframework.transaction.support.TransactionSynchronization;
import org.springframework.transaction.support.TransactionSynchronizationManager;
import org.springframework.web.client.RestClientException;

import java.math.BigDecimal;
import java.util.LinkedHashMap;
import java.util.LinkedHashSet;
import java.util.Map;
import java.util.Set;
import java.util.concurrent.ThreadLocalRandom;

/**
 * Default implementation of card-request flows backed by verification-service.
 */
@Service
@RequiredArgsConstructor
public class CardRequestServiceImpl implements CardRequestService {

    private static final CardBrand[] AUTOMATIC_CARD_BRANDS = CardBrand.values();

    private final CardCreationService cardCreationService;
    private final CardRepository cardRepository;
    private final AuthorizedPersonRepository authorizedPersonRepository;
    private final ClientService clientService;
    private final VerificationService verificationService;
    private final RabbitClient rabbitClient;
    private final CardCreationResponseMapper cardCreationResponseMapper;

    @Value("${card.creation.automatic.default-limit}")
    private BigDecimal automaticCardDefaultLimit;

    @Override
    @Transactional
    public CardCreationResponseDto createAutomaticCard(AutoCardCreationRequestDto request) {
        requireText(request.getAccountNumber(), ErrorCode.INVALID_ACCOUNT_NUMBER, "Account number must not be blank.");
        if (request.getClientId() == null) {
            throw new BusinessException(ErrorCode.INVALID_CLIENT_ID, "Client ID must be provided.");
        }

        CardCreationResult result = cardCreationService.createCard(new CreateCardCommand(
                request.getAccountNumber(),
                randomAutomaticCardBrand(),
                automaticCardDefaultLimit,
                request.getClientId(),
                null
        ));
        return cardCreationResponseMapper.toDto(result);
    }

    /**
     * Creates a personal card after the caller completes verification in verification-service.
     *
     * <p>The request must contain full card data together with a verification session ID.
     * Card-service no longer stores or validates a local personal verification code.
     *
     * @param accountContext resolved account owner and ownership type
     * @param request personal-card request DTO
     * @return completed response with the newly created card
     */
    @Override
    @Transactional
    public CardRequestResponseDto processManualCardRequest(
            AccountNotificationContextDto accountContext,
            ClientCardRequestDto request
    ) {
        validateInitiationRequest(request.getAccountNumber(), request.getCardBrand(), request.getCardLimit());
        String accountNumber = request.getAccountNumber().strip();
        Long ownerClientId = requireOwnerClientId(accountContext);

        if (accountContext.isBusinessAccount()) {
            throw new BusinessException(
                    ErrorCode.INVALID_ACCOUNT_TYPE,
                    "Business accounts must use the /request/business endpoint."
            );
        }
        ensureVerificationIsVerified(request.getVerificationId());

        try {
            enforcePersonalLimitOf2Accounts(accountNumber, ownerClientId);
            CardCreationResult result = cardCreationService.createCard(new CreateCardCommand(
                    accountNumber,
                    request.getCardBrand(),
                    request.getCardLimit(),
                    ownerClientId,
                    null
            ));
            ClientNotificationRecipientDto ownerRecipient = clientService.getNotificationRecipient(ownerClientId);
            registerAfterCommitRequestSuccess(ownerRecipient, null, result.card());

            return new CardRequestResponseDto(
                    "COMPLETED",
                    "Card created successfully.",
                    null,
                    cardCreationResponseMapper.toDto(result)
            );
        } catch (RuntimeException ex) {
            if (ex instanceof BusinessException businessException) {
                throw businessException;
            }
            throw new BusinessException(
                    ErrorCode.CARD_REQUEST_COMPLETION_FAILED,
                    "Unable to complete personal card request."
            );
        }
    }

    /**
     * Creates a business card after the caller completes verification in verification-service.
     *
     * <p>The request must contain full card data together with a verification session ID.
     * Card-service no longer stores or validates a local business verification request.
     *
     * @param accountContext resolved account owner and ownership type
     * @param request business-card request DTO
     * @return completed response with the newly created card
     */
    @Override
    @Transactional
    public CardRequestResponseDto processBusinessCardRequest(
            AccountNotificationContextDto accountContext,
            BusinessCardRequestDto request
    ) {
        validateInitiationRequest(request.getAccountNumber(), request.getCardBrand(), request.getCardLimit());
        String accountNumber = request.getAccountNumber().strip();
        Long ownerClientId = requireOwnerClientId(accountContext);
        if (request.getRecipientType() == null) {
            throw new BusinessException(ErrorCode.INVALID_REQUEST_STATE, "Recipient type must be provided.");
        }

        if (!accountContext.isBusinessAccount()) {
            throw new BusinessException(
                    ErrorCode.INVALID_ACCOUNT_TYPE,
                    "Personal accounts must use the /request endpoint."
            );
        }
        ensureVerificationIsVerified(request.getVerificationId());

        try {
            AuthorizedPerson authorizedPerson = resolveExistingAuthorizedPerson(request);
            if (request.getRecipientType() == CardRequestRecipientType.AUTHORIZED_PERSON && authorizedPerson == null) {
                authorizedPerson = createAuthorizedPerson(request.getAuthorizedPerson());
            }
            Long authorizedPersonId = request.getRecipientType() == CardRequestRecipientType.AUTHORIZED_PERSON
                    ? authorizedPerson == null ? null : authorizedPerson.getId()
                    : null;
            enforceBusinessLimit(accountNumber, ownerClientId, authorizedPersonId);

            CardCreationResult result = cardCreationService.createCard(new CreateCardCommand(
                    accountNumber,
                    request.getCardBrand(),
                    request.getCardLimit(),
                    ownerClientId,
                    authorizedPersonId
            ));

            if (authorizedPerson != null) {
                authorizedPerson.getCardIds().add(result.card().getId());
                authorizedPersonRepository.save(authorizedPerson);
            }

            ClientNotificationRecipientDto ownerRecipient = clientService.getNotificationRecipient(ownerClientId);
            NotificationRecipient authorizedRecipient = authorizedPerson == null ? null : toNotificationRecipient(authorizedPerson);
            registerAfterCommitRequestSuccess(ownerRecipient, authorizedRecipient, result.card());

            return new CardRequestResponseDto(
                    "COMPLETED",
                    "Card created successfully.",
                    null,
                    cardCreationResponseMapper.toDto(result)
            );
        } catch (RuntimeException ex) {
            if (ex instanceof BusinessException businessException) {
                throw businessException;
            }
            throw new BusinessException(
                    ErrorCode.CARD_REQUEST_COMPLETION_FAILED,
                    "Unable to complete business card request."
            );
        }
    }

    /**
     * Validates the common initiation payload fields shared by personal and business request flows.
     *
     * <p>The initiation step requires a non-blank account number, an explicit card brand,
     * and a non-negative card limit before any account or ownership checks are performed.
     *
     * @param accountNumber target account number
     * @param cardBrand requested card brand
     * @param cardLimit requested spending limit
     */
    private void validateInitiationRequest(String accountNumber, CardBrand cardBrand, BigDecimal cardLimit) {
        requireText(accountNumber, ErrorCode.INVALID_ACCOUNT_NUMBER, "Account number must not be blank.");
        if (cardBrand == null) {
            throw new BusinessException(ErrorCode.INVALID_CARD_BRAND, "Card brand must be provided.");
        }
        if (cardLimit == null || cardLimit.signum() < 0) {
            throw new BusinessException(ErrorCode.INVALID_CARD_LIMIT, "Card limit must be zero or greater.");
        }
    }

    private void ensureVerificationIsVerified(Long verificationId) {
        if (verificationId == null) {
            throw new BusinessException(ErrorCode.INVALID_REQUEST_STATE, "Verification ID must be provided.");
        }

        try {
            var verificationStatus = verificationService.getStatus(verificationId);
            if (verificationStatus == null || verificationStatus.status() != VerificationStatus.VERIFIED) {
                throw new BusinessException(ErrorCode.INVALID_REQUEST_STATE, "Verification is not completed.");
            }
        } catch (BusinessException ex) {
            throw ex;
        } catch (RestClientException ex) {
            throw new BusinessException(ErrorCode.INVALID_REQUEST_STATE, "Verification is not completed.");
        }
    }

    private Long requireOwnerClientId(AccountNotificationContextDto accountContext) {
        if (accountContext == null || accountContext.ownerClientId() == null) {
            throw new BusinessException(ErrorCode.INVALID_CLIENT_ID, "Account owner client ID is missing.");
        }
        return accountContext.ownerClientId();
    }

    /**
     * Enforces the personal-account rule: a client can have at most 2 non-deactivated cards per account.

     * The repository query counts active cards that belong to:
     * - the same account
     * - the same owner client
     * - no authorized person
     * - status other than {@link CardStatus#DEACTIVATED}
     *
     * This check is executed during both initiation and completion.

     * Example:
     * - count = 0 or 1: request may continue
     * - count = 2 or more: throws {@link BusinessException} with {@link ErrorCode#MAX_CARD_LIMIT_REACHED}
     *
     * @param accountNumber linked account number
     * @param clientId owner client ID
     */
    private void enforcePersonalLimitOf2Accounts(String accountNumber, Long clientId) {
        long count = cardRepository.countByAccountNumberAndClientIdAndAuthorizedPersonIdIsNullAndStatusNot(
                accountNumber.strip(),
                clientId,
                CardStatus.DEACTIVATED
        );
        if (count >= 2) {
            throw new BusinessException(
                    ErrorCode.MAX_CARD_LIMIT_REACHED,
                    "Personal accounts can have at most 2 active cards."
            );
        }
    }

    /**
     * Enforces the business-account rule: each person may have at most 1 non-deactivated card per account.

     * The query path depends on the target person:
     * - when {@code authorizedPersonId == null}, the card is for the business owner,
     *   so the count is performed by {@code accountNumber + clientId}
     * - when {@code authorizedPersonId != null}, the card is for an authorized person,
     *   so the count is performed by {@code accountNumber + authorizedPersonId}
     *
     * This check is executed during both initiation and completion.

     * Example:
     * - owner has 0 cards on that account: request may continue
     * - authorized person already has 1 active card on that account: throws
     *
     * @param accountNumber linked business account number
     * @param clientId owner client ID
     * @param authorizedPersonId optional authorized-person ID
     */
    private void enforceBusinessLimit(String accountNumber, Long clientId, Long authorizedPersonId) {
        long count = authorizedPersonId == null
                ? cardRepository.countByAccountNumberAndClientIdAndAuthorizedPersonIdIsNullAndStatusNot(
                        accountNumber.strip(),
                        clientId,
                        CardStatus.DEACTIVATED
                )
                : cardRepository.countByAccountNumberAndAuthorizedPersonIdAndStatusNot(
                        accountNumber.strip(),
                        authorizedPersonId,
                        CardStatus.DEACTIVATED
                );
        if (count >= 1) {
            throw new BusinessException(
                    ErrorCode.MAX_CARD_LIMIT_REACHED,
                    "Business accounts can have at most 1 active card per person."
            );
        }
    }

    /**
     * Resolves the authorized person referenced by a business request, when the recipient type requires one.

     * Resolution strategy:
     * - if recipient type is not {@code AUTHORIZED_PERSON}, returns {@code null}
     * - if {@code authorizedPersonId} is provided, loads that existing authorized person
     * - otherwise, expects an inline {@code authorizedPerson} payload and tries to find
     *   an existing record by email + first name + last name + date of birth
     * - if no match exists, returns {@code null}, which means a new authorized person will be created later

     * Example:
     * a request may reference an existing person by ID, or provide full identity data for lookup.
     *
     * @param request business-card request payload
     * @return resolved existing authorized person, or {@code null} when a new person must be created later
     */
    private AuthorizedPerson resolveExistingAuthorizedPerson(BusinessCardRequestDto request) {
        if (request.getRecipientType() != CardRequestRecipientType.AUTHORIZED_PERSON) {
            return null;
        }

        if (request.getAuthorizedPersonId() != null) {
            return authorizedPersonRepository.findById(request.getAuthorizedPersonId())
                    .orElseThrow(() -> new BusinessException(
                            ErrorCode.AUTHORIZED_PERSON_NOT_FOUND,
                            "Authorized person with ID " + request.getAuthorizedPersonId() + " was not found."
                    ));
        }

        AuthorizedPersonRequestDto authorizedPerson = request.getAuthorizedPerson();
        if (authorizedPerson == null) {
            throw new BusinessException(
                    ErrorCode.INVALID_REQUEST_STATE,
                    "Authorized-person details must be provided for this request."
            );
        }

        return authorizedPersonRepository.findByEmailIgnoreCaseAndFirstNameIgnoreCaseAndLastNameIgnoreCaseAndDateOfBirth(
                authorizedPerson.getEmail().trim(),
                authorizedPerson.getFirstName().trim(),
                authorizedPerson.getLastName().trim(),
                authorizedPerson.getDateOfBirth()
        ).orElse(null);
    }

    /**
     * Creates a new authorized person from the inline business-card request payload.
     *
     * @param source inline authorized-person payload from the request
     * @return newly persisted authorized person
     */
    private AuthorizedPerson createAuthorizedPerson(AuthorizedPersonRequestDto source) {
        if (source == null) {
            throw new BusinessException(
                    ErrorCode.INVALID_REQUEST_STATE,
                    "Authorized-person details must be provided for this request."
            );
        }
        AuthorizedPerson authorizedPerson = new AuthorizedPerson();
        authorizedPerson.setFirstName(source.getFirstName().trim());
        authorizedPerson.setLastName(source.getLastName().trim());
        authorizedPerson.setDateOfBirth(source.getDateOfBirth());
        authorizedPerson.setGender(source.getGender());
        authorizedPerson.setEmail(source.getEmail().trim());
        authorizedPerson.setPhone(source.getPhone().trim());
        authorizedPerson.setAddress(source.getAddress().trim());
        return authorizedPersonRepository.save(authorizedPerson);
    }

    /**
     * Schedules success notifications after the transaction commits successfully.

     * In the personal flow this is sent only to the owner.
     * In the business-authorized-person flow it may be sent to both the owner and the authorized person.

     * Why after commit:
     * the notification must reflect a card that was actually persisted.
     *
     * @param ownerRecipient owner notification recipient
     * @param authorizedRecipient optional authorized-person recipient
     * @param card newly created persisted card
     */
    private void registerAfterCommitRequestSuccess(
            ClientNotificationRecipientDto ownerRecipient,
            NotificationRecipient authorizedRecipient,
            Card card
    ) {
        Set<NotificationRecipient> recipients = new LinkedHashSet<>();
        recipients.add(new NotificationRecipient(ownerRecipient.displayName(), ownerRecipient.email()));
        if (authorizedRecipient != null) {
            recipients.add(authorizedRecipient);
        }
        TransactionSynchronizationManager.registerSynchronization(new TransactionSynchronization() {
            @Override
            public void afterCommit() {
                recipients.forEach(recipient -> rabbitClient.sendCardNotification(
                        CardNotificationType.CARD_REQUEST_SUCCESS,
                        new CardNotificationDto(
                                recipient.name(),
                                recipient.email(),
                                successTemplateVariables(card)
                        )
                ));
            }
        });
    }

    /**
     * Builds the template-variable map for success notifications after card creation completes.
     *
     * @param card newly created card
     * @return ordered template-variable map for the success notification
     */
    private Map<String, String> successTemplateVariables(Card card) {
        Map<String, String> variables = new LinkedHashMap<>();
        variables.put("cardNumber", SensitiveDataMasker.maskCardNumber(card.getCardNumber()));
        variables.put("accountNumber", SensitiveDataMasker.maskAccountNumber(card.getAccountNumber()));
        variables.put("cardName", card.getCardName());
        return variables;
    }

    /**
     * Requires a non-blank text value and throws the supplied business error when validation fails.
     *
     * @param value text value to validate
     * @param errorCode application-specific error code to throw on failure
     * @param message error message to propagate on failure
     */
    private void requireText(String value, ErrorCode errorCode, String message) {
        if (value == null || value.isBlank()) {
            throw new BusinessException(errorCode, message);
        }
    }

    /**
     * Adapts an authorized-person entity to the lightweight notification-recipient shape.
     *
     * @param authorizedPerson resolved authorized person entity
     * @return notification recipient containing display name and email
     */
    private NotificationRecipient toNotificationRecipient(AuthorizedPerson authorizedPerson) {
        return new NotificationRecipient(
                (authorizedPerson.getFirstName() + " " + authorizedPerson.getLastName()).trim(),
                authorizedPerson.getEmail()
        );
    }

    /**
     * @return random card type (VISA, MASTERCARD...), any random type of this 4 types
     */
    private CardBrand randomAutomaticCardBrand() {
        return AUTOMATIC_CARD_BRANDS[ThreadLocalRandom.current().nextInt(AUTOMATIC_CARD_BRANDS.length)];
    }

    /**
     * Lightweight in-memory representation of a notification recipient used during request flows.
     *
     * @param name recipient display name
     * @param email recipient email address
     */
    private record NotificationRecipient(String name, String email) {
    }
}
