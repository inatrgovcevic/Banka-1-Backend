package com.banka1.verificationService.controller;

import com.banka1.verificationService.dto.request.GenerateRequest;
import com.banka1.verificationService.dto.request.ValidateRequest;
import com.banka1.verificationService.dto.response.GenerateResponse;
import com.banka1.verificationService.dto.response.StatusResponse;
import com.banka1.verificationService.dto.response.ValidateResponse;
import com.banka1.verificationService.service.VerificationService;
import jakarta.validation.Valid;
import lombok.RequiredArgsConstructor;
import org.springframework.http.ResponseEntity;
import org.springframework.security.core.annotation.AuthenticationPrincipal;
import org.springframework.security.oauth2.jwt.Jwt;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.PathVariable;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.RequestBody;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;

/**
 * REST controller for managing verification session operations.
 * Provides endpoints for generating, validating, and checking verification codes.
 * All operations require JWT authentication via Spring Security.
 *
 * This controller orchestrates two-factor authentication (2FA) by delegating
 * business logic to {@link VerificationService}.
 */
@RestController
@RequestMapping("/verification")
@RequiredArgsConstructor
public class VerificationController {

    /** Service that handles business logic for verification operations. */
    private final VerificationService verificationService;

    /**
     * Generates a new verification session and sends a one-time password (OTP) code.
     *
     * Cancels any existing PENDING sessions for the same client, operation type,
     * and related entity, then creates a new session with a freshly generated
     * 6-digit OTP code that will be hashed and stored. The raw code is sent
     * via RabbitMQ to the notification service for email delivery.
     *
     * @param jwt JWT token containing authenticated user information (not currently used)
     * @param request contains clientId, operationType, relatedEntityId, and clientEmail
     * @return response containing the newly generated session ID
     * @throws BusinessException if clientId does not match authenticated user
     * @throws BusinessException if a PENDING session already exists for the same combination
     */
    @PostMapping("/generate")
    public ResponseEntity<GenerateResponse> generate(@AuthenticationPrincipal Jwt jwt, @Valid @RequestBody GenerateRequest request) {
        return ResponseEntity.ok(verificationService.generate(request));
    }

    /**
     * Validates a verification code against an existing session.
     *
     * Checks if the provided code matches the hashed code stored in the session.
     * Increments the attempt counter on mismatch and cancels the session after
     * 3 failed attempts. Updates session status to VERIFIED on success or
     * EXPIRED/CANCELLED as appropriate.
     *
     * @param jwt JWT token containing authenticated user information (not currently used)
     * @param request contains sessionId and the user-provided code
     * @return response indicating whether validation succeeded, current session status,
     *         and remaining attempts before session cancellation
     * @throws BusinessException if session not found
     * @throws BusinessException if session is already VERIFIED, CANCELLED, or EXPIRED
     */
    @PostMapping("/validate")
    public ResponseEntity<ValidateResponse> validate(@AuthenticationPrincipal Jwt jwt, @Valid @RequestBody ValidateRequest request) {
        return ResponseEntity.ok(verificationService.validate(request));
    }

    /**
     * Retrieves the current status of a verification session.
     *
     * If the session is PENDING and past its expiration time, automatically
     * transitions it to EXPIRED status before returning.
     *
     * @param jwt JWT token containing authenticated user information (not currently used)
     * @param sessionId the ID of the session to check
     * @return response containing the session ID and its current status
     * @throws BusinessException if session not found
     */
    @GetMapping("/{sessionId}/status")
    public ResponseEntity<StatusResponse> getStatus(@AuthenticationPrincipal Jwt jwt, @PathVariable Long sessionId) {
        return ResponseEntity.ok(verificationService.getStatus(sessionId));
    }
}
