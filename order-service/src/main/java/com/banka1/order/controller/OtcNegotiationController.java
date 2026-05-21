package com.banka1.order.controller;

import com.banka1.order.dto.AuthenticatedUser;
import com.banka1.order.dto.CreateOtcNegotiationRequest;
import com.banka1.order.dto.OtcNegotiationHistoryResponse;
import com.banka1.order.dto.OtcNegotiationResponse;
import com.banka1.order.dto.UpdateOtcNegotiationRequest;
import com.banka1.order.entity.enums.OtcNegotiationStatus;
import com.banka1.order.service.OtcNegotiationService;
import jakarta.validation.Valid;
import lombok.RequiredArgsConstructor;
import org.springframework.http.ResponseEntity;
import org.springframework.security.access.prepost.PreAuthorize;
import org.springframework.security.core.annotation.AuthenticationPrincipal;
import org.springframework.security.oauth2.jwt.Jwt;
import org.springframework.web.bind.annotation.*;

import java.time.LocalDate;
import java.util.Collection;
import java.util.LinkedHashSet;
import java.util.List;
import java.util.Set;

/**
 * REST endpoints for OTC negotiation workflow and history.
 */
@RestController
@RequestMapping("/otc-negotiations")
@RequiredArgsConstructor
public class OtcNegotiationController {

    private final OtcNegotiationService otcNegotiationService;

    @PostMapping
    @PreAuthorize("hasAnyRole('CLIENT_TRADING','AGENT','SUPERVISOR')")
    public ResponseEntity<OtcNegotiationResponse> createNegotiation(
            @AuthenticationPrincipal Jwt jwt,
            @Valid @RequestBody CreateOtcNegotiationRequest request
    ) {
        return ResponseEntity.ok(otcNegotiationService.createNegotiation(toAuthenticatedUser(jwt), request));
    }

    @PostMapping("/{id}/counteroffer")
    @PreAuthorize("hasAnyRole('CLIENT_TRADING','AGENT','SUPERVISOR')")
    public ResponseEntity<OtcNegotiationResponse> counterOffer(
            @AuthenticationPrincipal Jwt jwt,
            @PathVariable Long id,
            @Valid @RequestBody UpdateOtcNegotiationRequest request
    ) {
        return ResponseEntity.ok(otcNegotiationService.counterOffer(toAuthenticatedUser(jwt), id, request));
    }

    @PostMapping("/{id}/accept")
    @PreAuthorize("hasAnyRole('CLIENT_TRADING','AGENT','SUPERVISOR')")
    public ResponseEntity<OtcNegotiationResponse> acceptNegotiation(
            @AuthenticationPrincipal Jwt jwt,
            @PathVariable Long id
    ) {
        return ResponseEntity.ok(otcNegotiationService.acceptNegotiation(toAuthenticatedUser(jwt), id));
    }

    @PostMapping("/{id}/decline")
    @PreAuthorize("hasAnyRole('CLIENT_TRADING','AGENT','SUPERVISOR')")
    public ResponseEntity<OtcNegotiationResponse> declineNegotiation(
            @AuthenticationPrincipal Jwt jwt,
            @PathVariable Long id
    ) {
        return ResponseEntity.ok(otcNegotiationService.declineNegotiation(toAuthenticatedUser(jwt), id));
    }

    @PostMapping("/{id}/cancel")
    @PreAuthorize("hasAnyRole('CLIENT_TRADING','AGENT','SUPERVISOR')")
    public ResponseEntity<OtcNegotiationResponse> cancelNegotiation(
            @AuthenticationPrincipal Jwt jwt,
            @PathVariable Long id
    ) {
        return ResponseEntity.ok(otcNegotiationService.cancelNegotiation(toAuthenticatedUser(jwt), id));
    }

    @GetMapping
    @PreAuthorize("hasAnyRole('CLIENT_TRADING','AGENT','SUPERVISOR')")
    public ResponseEntity<List<OtcNegotiationResponse>> listNegotiations(
            @AuthenticationPrincipal Jwt jwt,
            @RequestParam(required = false) OtcNegotiationStatus status,
            @RequestParam(required = false) LocalDate dateFrom,
            @RequestParam(required = false) LocalDate dateTo,
            @RequestParam(required = false) Long counterpartyId
    ) {
        return ResponseEntity.ok(otcNegotiationService.listNegotiations(
                toAuthenticatedUser(jwt), status, dateFrom, dateTo, counterpartyId
        ));
    }

    @GetMapping("/{id}/history")
    @PreAuthorize("hasAnyRole('CLIENT_TRADING','AGENT','SUPERVISOR')")
    public ResponseEntity<List<OtcNegotiationHistoryResponse>> getHistory(
            @AuthenticationPrincipal Jwt jwt,
            @PathVariable Long id
    ) {
        return ResponseEntity.ok(otcNegotiationService.getNegotiationHistory(toAuthenticatedUser(jwt), id));
    }

    private AuthenticatedUser toAuthenticatedUser(Jwt jwt) {
        Object idClaim = jwt.getClaim("id");
        Long id = idClaim != null
                ? ((Number) idClaim).longValue()
                : Long.valueOf(jwt.getSubject());
        return new AuthenticatedUser(
                id,
                extractStrings(jwt.getClaim("roles")),
                extractStrings(jwt.getClaim("permissions"))
        );
    }

    private Set<String> extractStrings(Object claim) {
        if (claim == null) {
            return Set.of();
        }
        if (claim instanceof String value) {
            return Set.of(value);
        }
        if (claim instanceof Collection<?> values) {
            Set<String> result = new LinkedHashSet<>();
            for (Object value : values) {
                if (value != null) {
                    result.add(String.valueOf(value));
                }
            }
            return result;
        }
        return Set.of(String.valueOf(claim));
    }
}
