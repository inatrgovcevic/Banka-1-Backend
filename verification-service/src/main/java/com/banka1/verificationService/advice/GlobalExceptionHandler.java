package com.banka1.verificationService.advice;

import com.banka1.verificationService.dto.response.ErrorResponseDto;
import com.banka1.verificationService.exception.BusinessException;
import org.springframework.amqp.AmqpException;
import org.springframework.dao.DataIntegrityViolationException;
import org.springframework.http.HttpStatus;
import org.springframework.http.ResponseEntity;
import org.springframework.security.access.AccessDeniedException;
import org.springframework.security.authorization.AuthorizationDeniedException;
import org.springframework.stereotype.Component;
import org.springframework.validation.FieldError;
import org.springframework.web.bind.MethodArgumentNotValidException;
import org.springframework.web.bind.annotation.ExceptionHandler;
import org.springframework.web.bind.annotation.RestControllerAdvice;

import java.util.HashMap;
import java.util.Map;

/**
 * Centralized exception handler for all REST controllers in the verification service.
 *
 * Implements Spring's {@link org.springframework.web.bind.annotation.RestControllerAdvice}
 * to intercept exceptions thrown by controller methods and convert them into standardized
 * HTTP responses with {@link ErrorResponseDto} bodies. Handles both expected (business)
 * exceptions and unexpected runtime exceptions.
 *
 * Exception handling strategy:
 * <ul>
 *   <li>Business exceptions → HTTP status and error code from {@link ErrorCode} enum</li>
 *   <li>Validation exceptions → HTTP 400 Bad Request with field-level errors</li>
 *   <li>Data integrity violations → HTTP 409 Conflict</li>
 *   <li>Access denied → HTTP 403 Forbidden</li>
 *   <li>Messaging failures → HTTP 500 Internal Server Error</li>
 *   <li>Unexpected exceptions → HTTP 500 with generic error message</li>
 * </ul>
 */
@RestControllerAdvice
@Component("verificationServiceGlobalExceptionHandler")
public class GlobalExceptionHandler {

    /**
     * Handles application business exceptions.
     *
     * Extracts the error code, title, and details from the {@link BusinessException}
     * and returns them with the appropriate HTTP status.
     *
     * @param ex the business exception thrown by service logic
     * @return HTTP response with error code and details, status determined by ErrorCode
     */
    @ExceptionHandler(BusinessException.class)
    public ResponseEntity<ErrorResponseDto> handleBusinessException(BusinessException ex) {
        ErrorResponseDto error = new ErrorResponseDto(
                ex.getErrorCode().getCode(),
                ex.getErrorCode().getTitle(),
                ex.getDetails()
        );
        return new ResponseEntity<>(error, ex.getErrorCode().getHttpStatus());
    }

    /**
     * Handles database integrity constraint violations.
     *
     * Catches violations such as unique constraint conflicts, foreign key violations,
     * and other database-level integrity errors. Returns a generic conflict response.
     *
     * @param ex the exception thrown by Spring Data when a constraint is violated
     * @return HTTP 409 Conflict response
     */
    @ExceptionHandler(DataIntegrityViolationException.class)
    public ResponseEntity<ErrorResponseDto> handleDataIntegrityViolation(DataIntegrityViolationException ex) {
        ErrorResponseDto error = new ErrorResponseDto(
                "ERR_CONSTRAINT_VIOLATION",
                "Podatak već postoji",
                "Jedan od podataka je već u upotrebi."
        );
        return new ResponseEntity<>(error, HttpStatus.CONFLICT);
    }

    /**
     * Handles programmatic validation failures.
     *
     * Catches exceptions thrown by explicit validation checks in code (e.g., manual
     * parameter validation). Returns a standardized bad request response.
     *
     * @param ex the exception containing validation error details
     * @return HTTP 400 Bad Request response
     */
    @ExceptionHandler(IllegalArgumentException.class)
    public ResponseEntity<ErrorResponseDto> handleIllegalArgument(IllegalArgumentException ex) {
        ErrorResponseDto error = new ErrorResponseDto(
                "ERR_VALIDATION",
                "Neispravni argumenti",
                ex.getMessage()
        );
        return new ResponseEntity<>(error, HttpStatus.BAD_REQUEST);
    }

    /**
     * Handles JSR-380 bean validation failures.
     *
     * Catches {@link org.springframework.web.bind.MethodArgumentNotValidException}
     * thrown by Spring when request DTOs fail validation annotations (@NotNull, @Email, etc.).
     * Extracts field-level errors and returns them in a structured map.
     *
     * @param ex the exception containing field validation errors
     * @return HTTP 400 Bad Request response with a map of field errors
     */
    @ExceptionHandler(MethodArgumentNotValidException.class)
    public ResponseEntity<ErrorResponseDto> handleValidation(MethodArgumentNotValidException ex) {
        Map<String, String> validationErrors = new HashMap<>();
        for (FieldError fieldError : ex.getBindingResult().getFieldErrors()) {
            validationErrors.put(fieldError.getField(), fieldError.getDefaultMessage());
        }
        ErrorResponseDto error = new ErrorResponseDto(
                "ERR_VALIDATION",
                "Neispravni podaci",
                "Molimo Vas proverite unete podatke.",
                validationErrors
        );
        return new ResponseEntity<>(error, HttpStatus.BAD_REQUEST);
    }

    /**
     * Handles RabbitMQ message broker failures.
     *
     * Catches AMQP exceptions that occur when publishing verification events
     * (e.g., OTP codes for email delivery). Returns a generic internal server error
     * without exposing broker details to the client.
     *
     * @param ex the AMQP exception thrown during message publishing
     * @return HTTP 500 Internal Server Error response
     */
    @ExceptionHandler(AmqpException.class)
    public ResponseEntity<ErrorResponseDto> handleAmqpException(AmqpException ex) {
        ErrorResponseDto error = new ErrorResponseDto(
                "ERR_INTERNAL_SERVER",
                "Serverska greška",
                "Poruka nije dostavljena. Naš tim je obavešten."
        );
        return new ResponseEntity<>(error, HttpStatus.INTERNAL_SERVER_ERROR);
    }

    /**
     * Handles access denial exceptions.
     *
     * Catches {@link org.springframework.security.access.AccessDeniedException} and
     * {@link org.springframework.security.authorization.AuthorizationDeniedException}
     * thrown by Spring Security when @PreAuthorize checks fail or when a user
     * lacks required roles. Returns a generic forbidden response without exposing
     * authorization logic to the client.
     * <p>
     * Spring Security 5: @Secured / @PreAuthorize -> AccessDeniedException.
     * Spring Security 6: @PreAuthorize -> AuthorizationDeniedException (subclass of AccessDeniedException).
     *
     * @param ex the access denied exception
     * @return HTTP 403 Forbidden response
     */
    @ExceptionHandler({AccessDeniedException.class, AuthorizationDeniedException.class})
    public ResponseEntity<ErrorResponseDto> handleAccessDenied(AccessDeniedException ex) {
        ErrorResponseDto error = new ErrorResponseDto(
                "ERR_FORBIDDEN",
                "Pristup odbijen",
                "Nemate dozvolu za ovu akciju."
        );
        return new ResponseEntity<>(error, HttpStatus.FORBIDDEN);
    }

    /**
     * Handles unexpected exceptions as a fallback.
     *
     * Catches any {@link Exception} not handled by more specific handlers.
     * Returns a generic internal server error response without exposing the
     * exception stack trace or details to the client for security reasons.
     *
     * @param ex the unexpected exception
     * @return HTTP 500 Internal Server Error response
     */
    @ExceptionHandler(Exception.class)
    public ResponseEntity<ErrorResponseDto> handleUnexpectedException(Exception ex) {
        ErrorResponseDto error = new ErrorResponseDto(
                "ERR_INTERNAL_SERVER",
                "Serverska greška",
                "Došlo je do neočekivanog problema. Naš tim je obavešten."
        );
        return new ResponseEntity<>(error, HttpStatus.INTERNAL_SERVER_ERROR);
    }
}
