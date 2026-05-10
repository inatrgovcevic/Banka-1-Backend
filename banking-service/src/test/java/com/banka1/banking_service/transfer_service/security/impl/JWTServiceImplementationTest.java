package com.banka1.banking_service.transfer_service.security.impl;

import com.banka1.banking_service.transfer_service.security.impl.JWTServiceImplementation;
import com.nimbusds.jwt.SignedJWT;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.springframework.test.util.ReflectionTestUtils;

import java.text.ParseException;

import static org.junit.jupiter.api.Assertions.*;

class JWTServiceImplementationTest {

    private JWTServiceImplementation jwtService;
    private final String secret = "OvoMoraBitiJakoDugacakSecretDaBiMACSignerBioSrecan123!";

    @BeforeEach
    void setUp() throws Exception {
        jwtService = new JWTServiceImplementation(secret);
        // Postavljamo @Value polja jer Spring nije prisutan u Unit testu
        ReflectionTestUtils.setField(jwtService, "roleClaim", "roles");
        ReflectionTestUtils.setField(jwtService, "permissionClaim", "permissions");
        ReflectionTestUtils.setField(jwtService, "issuer", "banka1");
        ReflectionTestUtils.setField(jwtService, "expirationTime", 3600000L);
    }

    @Test
    void generateJwtToken_ShouldCreateValidTokenWithServiceRole() throws ParseException {
        // Act
        String token = jwtService.generateJwtToken();

        // Assert
        assertNotNull(token);

        // Dekodiranje tokena za provere
        SignedJWT decodedJwt = SignedJWT.parse(token);
        assertEquals("transfer-service", decodedJwt.getJWTClaimsSet().getSubject());
        assertEquals("banka1", decodedJwt.getJWTClaimsSet().getIssuer());

        // Provera uloge
        Object roles = decodedJwt.getJWTClaimsSet().getClaim("roles");
        // Može biti String ili List zavisno od implementacije, tvoj kod stavlja "SERVICE" (String)
        assertEquals("SERVICE", roles.toString());

        // Provera da token nije istekao
        assertTrue(decodedJwt.getJWTClaimsSet().getExpirationTime().after(new java.util.Date()));
    }
}
