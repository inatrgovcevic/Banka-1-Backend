package com.banka1.banking_service.card_service.controller;

import com.banka1.banking_service.card_service.exception.BusinessException;
import com.banka1.banking_service.card_service.exception.ErrorCode;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.security.core.Authentication;
import org.springframework.security.core.GrantedAuthority;
import org.springframework.security.oauth2.jwt.Jwt;
import org.springframework.stereotype.Component;

/**
 * Shared controller support for extracting authenticated client data and ownership checks.
 */
@Component
public class CardControllerSupport {

    @Value("${banka.security.id}")
    private String jwtIdClaim;

    public void verifyOwnership(Jwt jwt, Long resourceOwnerClientId) {
        verifyOwnership(jwt, resourceOwnerClientId, "You do not own this resource.");
    }

    public void verifyOwnership(Jwt jwt, Long resourceOwnerClientId, String message) {
        Long requestingClientId = extractClientId(jwt);
        if (!requestingClientId.equals(resourceOwnerClientId)) {
            throw new BusinessException(ErrorCode.ACCESS_DENIED, message);
        }
    }

    public void verifyOwnershipIfClient(
            Authentication authentication,
            Jwt jwt,
            Long resourceOwnerClientId,
            String message
    ) {
        if (isClient(authentication)) {
            verifyOwnership(jwt, resourceOwnerClientId, message);
        }
    }

    public Long extractClientId(Jwt jwt) {
        return ((Number) jwt.getClaim(jwtIdClaim)).longValue();
    }

    public boolean isClient(Authentication authentication) {
        if (authentication == null) {
            return false;
        }
        return authentication.getAuthorities().stream()
                .map(GrantedAuthority::getAuthority)
                .anyMatch(authority -> authority.startsWith("ROLE_CLIENT_"));
    }
}
