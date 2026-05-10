package com.banka1.banking_service.card_service.exception;

import lombok.Getter;
import org.springframework.http.HttpStatus;

/**
 * Centralized catalog of business errors exposed by the card service.
 * Each constant defines the HTTP status returned to clients, a stable machine-readable error code,
 * and a short human-readable title.
 */
@Getter
public enum ErrorCode {

    /**
     * Returned when the caller provides a blank or missing account number.
     */
    INVALID_ACCOUNT_NUMBER(HttpStatus.BAD_REQUEST, "ERR_CARD_001", "Invalid account number"),

    /**
     * Returned when the caller provides a negative or missing card limit.
     */
    INVALID_CARD_LIMIT(HttpStatus.BAD_REQUEST, "ERR_CARD_002", "Invalid card limit"),

    /**
     * Returned when the service cannot generate a unique card number after repeated attempts.
     */
    CARD_NUMBER_GENERATION_FAILED(
            HttpStatus.INTERNAL_SERVER_ERROR,
            "ERR_CARD_003",
            "Card number generation failed"
    ),

    /**
     * Returned when no card exists for the given card number.
     */
    CARD_NOT_FOUND(HttpStatus.NOT_FOUND, "ERR_CARD_004", "Card not found"),

    /**
     * Returned when the requested status transition is not allowed by the state machine.
     */
    INVALID_STATUS_TRANSITION(HttpStatus.UNPROCESSABLE_ENTITY, "ERR_CARD_005", "Invalid status transition"),

    /**
     * Returned when a negative or null card limit is provided.
     */
    INVALID_LIMIT(HttpStatus.BAD_REQUEST, "ERR_CARD_006", "Invalid card limit"),

    /**
     * Returned when a client attempts to perform an action on a card they do not own.
     */
    ACCESS_DENIED(HttpStatus.FORBIDDEN, "ERR_CARD_007", "Access denied"),

    /**
     * Returned when the caller does not provide a supported card brand.
     */
    INVALID_CARD_BRAND(HttpStatus.BAD_REQUEST, "ERR_CARD_008", "Invalid card brand"),

    /**
     * Returned when the owner client ID is missing for internal card creation.
     */
    INVALID_CLIENT_ID(HttpStatus.BAD_REQUEST, "ERR_CARD_009", "Invalid client ID"),

    /**
     * Returned when the maximum number of allowed cards has already been reached.
     */
    MAX_CARD_LIMIT_REACHED(
            HttpStatus.UNPROCESSABLE_ENTITY,
            "ERR_CARD_010",
            "Maximum card limit reached"
    ),

    /**
     * Returned when a provided verification code does not match the stored request.
     */
    INVALID_VERIFICATION_CODE(
            HttpStatus.BAD_REQUEST,
            "ERR_CARD_011",
            "Invalid verification code"
    ),

    /**
     * Returned when a verification request cannot be found.
     */
    VERIFICATION_REQUEST_NOT_FOUND(
            HttpStatus.NOT_FOUND,
            "ERR_CARD_012",
            "Verification request not found"
    ),

    /**
     * Returned when a stored verification request is no longer valid.
     */
    VERIFICATION_REQUEST_EXPIRED(
            HttpStatus.UNPROCESSABLE_ENTITY,
            "ERR_CARD_013",
            "Verification request expired"
    ),

    /**
     * Returned when the requested authorized person does not exist.
     */
    AUTHORIZED_PERSON_NOT_FOUND(
            HttpStatus.NOT_FOUND,
            "ERR_CARD_014",
            "Authorized person not found"
    ),

    /**
     * Returned when request data does not match the expected flow state.
     */
    INVALID_REQUEST_STATE(
            HttpStatus.BAD_REQUEST,
            "ERR_CARD_015",
            "Invalid request state"
    ),

    /**
     * Returned when an endpoint is invoked for the wrong account type.
     */
    INVALID_ACCOUNT_TYPE(HttpStatus.UNPROCESSABLE_ENTITY, "ERR_CARD_016", "Invalid account type"),

    /**
     * Returned when personal card-request completion fails for an unexpected technical reason.
     */
    CARD_REQUEST_COMPLETION_FAILED(
            HttpStatus.INTERNAL_SERVER_ERROR,
            "ERR_CARD_017",
            "Card request completion failed"
    );

    /**
     * HTTP status mapped by the global exception handler.
     */
    private final HttpStatus httpStatus;

    /**
     * Stable machine-readable code intended for clients and integrations.
     */
    private final String code;

    /**
     * Short title suitable for API error responses.
     */
    private final String title;

    ErrorCode(HttpStatus httpStatus, String code, String title) {
        this.httpStatus = httpStatus;
        this.code = code;
        this.title = title;
    }
}
