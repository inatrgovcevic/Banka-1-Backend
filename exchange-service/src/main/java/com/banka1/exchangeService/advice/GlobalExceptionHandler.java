package com.banka1.exchangeService.advice;

import com.banka1.exchangeService.dto.ErrorResponseDto;
import com.banka1.exchangeService.exception.BusinessException;
import com.banka1.exchangeService.exception.ErrorCode;
import lombok.extern.slf4j.Slf4j;
import org.springframework.http.HttpStatus;
import org.springframework.http.ResponseEntity;
import org.springframework.security.access.AccessDeniedException;
import org.springframework.security.authorization.AuthorizationDeniedException;
import org.springframework.stereotype.Component;
import org.springframework.validation.FieldError;
import org.springframework.validation.BindException;
import org.springframework.web.bind.MethodArgumentNotValidException;
import org.springframework.web.bind.annotation.ExceptionHandler;
import org.springframework.web.bind.annotation.RestControllerAdvice;

import java.util.HashMap;
import java.util.Map;

/**
 * Centralized exception handler for the REST controller layer of the exchange-service.
 * Converts various exception types into standardized JSON error responses with
 * appropriate HTTP status codes.
 */
@Slf4j
@RestControllerAdvice
@Component("exchangeServiceGlobalExceptionHandler")
public class GlobalExceptionHandler {

    /**
     * Handles business logic exceptions and maps them to appropriate HTTP responses.
     *
     * @param ex domain-specific business exception
     * @return standardized error response payload
     */
    @ExceptionHandler(BusinessException.class)
    public ResponseEntity<ErrorResponseDto> handleBusinessException(BusinessException ex) {
        ErrorCode errorCode = ex.getErrorCode();
        ErrorResponseDto error = new ErrorResponseDto(
                errorCode.getCode(),
                errorCode.getTitle(),
                ex.getMessage()
        );
        return new ResponseEntity<>(error, errorCode.getHttpStatus());
    }

    /**
     * Handles request body validation errors and maps them to a structured response.
     *
     * @param ex validation exception from request body
     * @return payload with detailed field validation errors
     */
    @ExceptionHandler(MethodArgumentNotValidException.class)
    public ResponseEntity<ErrorResponseDto> handleValidation(MethodArgumentNotValidException ex) {
        return ResponseEntity.badRequest()
                .body(buildValidationErrorResponse(ex.getBindingResult().getFieldErrors()));
    }

    /**
     * Handles query parameter and model binding validation errors
     * and maps them to a structured response.
     *
     * @param ex binding exception from query parameters or model binding
     * @return payload with detailed field validation errors
     */
    @ExceptionHandler(BindException.class)
    public ResponseEntity<ErrorResponseDto> handleBindException(BindException ex) {
        return ResponseEntity.badRequest()
                .body(buildValidationErrorResponse(ex.getBindingResult().getFieldErrors()));
    }

    /**
     * Builds a validation error response from field validation errors.
     *
     * @param fieldErrors validation field errors
     * @return error response with field-level details
     */
    private ErrorResponseDto buildValidationErrorResponse(Iterable<FieldError> fieldErrors) {
        Map<String, String> validationErrors = new HashMap<>();
        for (FieldError fieldError : fieldErrors) {
            validationErrors.put(fieldError.getField(), fieldError.getDefaultMessage());
        }

        return new ErrorResponseDto(
                ErrorCode.VALIDATION_ERROR.getCode(),
                ErrorCode.VALIDATION_ERROR.getTitle(),
                "Molimo proverite unete podatke.",
                validationErrors
        );
    }

    /**
     * Catches unexpected exceptions and returns a generic 500 server error response.
     * Logs the full stack trace for diagnostic purposes.
     *
     * @param ex unexpected exception
     * @return generic error response payload
     */
    @ExceptionHandler(org.springframework.web.servlet.resource.NoResourceFoundException.class)
    public ResponseEntity<ErrorResponseDto> handleNoResource(org.springframework.web.servlet.resource.NoResourceFoundException ex) {
        return ResponseEntity.status(org.springframework.http.HttpStatus.NOT_FOUND)
                .body(new ErrorResponseDto("ERR_NOT_FOUND", "Resurs nije pronadjen", ex.getResourcePath()));
    }

    @ExceptionHandler(org.springframework.web.method.annotation.MethodArgumentTypeMismatchException.class)
    public ResponseEntity<ErrorResponseDto> handleTypeMismatch(org.springframework.web.method.annotation.MethodArgumentTypeMismatchException ex) {
        String requiredType = ex.getRequiredType() == null ? "" : ex.getRequiredType().getSimpleName();
        String detail = "Invalid value '" + ex.getValue() + "' for parameter '" + ex.getName() + "'"
                + (requiredType.isEmpty() ? "." : ", expected type: " + requiredType + ".");
        return ResponseEntity.badRequest()
                .body(new ErrorResponseDto("ERR_VALIDATION", "Neispravni argumenti", detail));
    }

    @ExceptionHandler(org.springframework.http.converter.HttpMessageNotReadableException.class)
    public ResponseEntity<ErrorResponseDto> handleNotReadable(org.springframework.http.converter.HttpMessageNotReadableException ex) {
        String detail = ex.getMostSpecificCause() != null ? ex.getMostSpecificCause().getMessage() : ex.getMessage();
        return ResponseEntity.badRequest()
                .body(new ErrorResponseDto("ERR_VALIDATION", "Neispravni podaci", detail));
    }

    /**
     * Handles Spring Security access denial.
     * <p>
     * Spring Security 5: @Secured / @PreAuthorize -> AccessDeniedException.
     * Spring Security 6: @PreAuthorize -> AuthorizationDeniedException (subclass of AccessDeniedException).
     * Explicit handler prevents fall-through to the generic Exception catch-all (which would return 500).
     *
     * @param ex access denied exception
     * @return HTTP 403 Forbidden response
     */
    @ExceptionHandler({AccessDeniedException.class, AuthorizationDeniedException.class})
    public ResponseEntity<ErrorResponseDto> handleAccessDenied(AccessDeniedException ex) {
        return ResponseEntity.status(HttpStatus.FORBIDDEN)
                .body(new ErrorResponseDto(
                        "ERR_FORBIDDEN",
                        "Pristup odbijen",
                        "Nemate dozvolu za ovu akciju."
                ));
    }

    @ExceptionHandler(Exception.class)
    public ResponseEntity<ErrorResponseDto> handleUnexpectedException(Exception ex) {
        log.error("Unexpected error in exchange-service", ex);
        return ResponseEntity.internalServerError()
                .body(new ErrorResponseDto(
                        "ERR_INTERNAL_SERVER",
                        "Serverska greska",
                        "Doslo je do neocekivane greske."
                ));
    }
}
