package com.banka1.banking_service.card_service.security.implementation;

import com.banka1.banking_service.card_service.security.JWTService;
import com.nimbusds.jose.JWSAlgorithm;
import com.nimbusds.jose.JWSHeader;
import com.nimbusds.jose.JWSSigner;
import com.nimbusds.jose.KeyLengthException;
import com.nimbusds.jose.crypto.MACSigner;
import com.nimbusds.jwt.JWTClaimsSet;
import com.nimbusds.jwt.SignedJWT;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.stereotype.Service;

import java.util.Date;
import java.util.List;

/**
 * HMAC-backed JWT generator for authenticated service-to-service requests.
 */
@Service
public class JWTServiceImplementation implements JWTService {

    private final JWSSigner signer;

    @Value("${banka.security.roles-claim}")
    private String roleClaim;

    @Value("${banka.security.permissions-claim}")
    private String permissionsClaim;

    @Value("${banka.security.issuer}")
    private String issuer;

    @Value("${banka.security.expiration-time}")
    private Long expirationTime;

    public JWTServiceImplementation(@Value("${jwt.secret}") String secret) throws KeyLengthException {
        this.signer = new MACSigner(secret);
    }

    @Override
    public String generateJwtToken() {
        JWTClaimsSet claims = new JWTClaimsSet.Builder()
                .subject("card-service")
                .issuer(issuer)
                .claim(roleClaim, "SERVICE")
                .claim(permissionsClaim, List.of())
                .expirationTime(new Date(System.currentTimeMillis() + expirationTime))
                .build();

        SignedJWT jwt = new SignedJWT(new JWSHeader(JWSAlgorithm.HS256), claims);
        try {
            jwt.sign(signer);
        } catch (Exception ex) {
            throw new IllegalStateException("Failed to generate service JWT", ex);
        }
        return jwt.serialize();
    }
}
