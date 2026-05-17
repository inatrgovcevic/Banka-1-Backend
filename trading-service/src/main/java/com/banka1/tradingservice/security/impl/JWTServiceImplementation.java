package com.banka1.tradingservice.security.impl;

import com.banka1.tradingservice.security.JWTService;
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

@Service
public class JWTServiceImplementation implements JWTService {

    private final JWSSigner signer;

    @Value("${banka.security.roles-claim:roles}")
    private String roleClaim;

    @Value("${banka.security.permissions-claim:permissions}")
    private String permissionClaim;

    @Value("${banka.security.issuer:banka1}")
    private String issuer;

    @Value("${banka.security.expiration-time:3600000}")
    private Long expirationTime;

    public JWTServiceImplementation(@Value("${jwt.secret}") String secret) throws KeyLengthException {
        this.signer = new MACSigner(secret);
    }

    @Override
    public String generateJwtToken() {
        JWTClaimsSet claims = new JWTClaimsSet.Builder()
                .subject("trading-service")
                .issuer(issuer)
                .claim(roleClaim, "SERVICE")
                .claim(permissionClaim, List.of())
                .expirationTime(new Date(System.currentTimeMillis() + expirationTime))
                .build();

        JWSHeader header = new JWSHeader(JWSAlgorithm.HS256);
        SignedJWT jwt = new SignedJWT(header, claims);
        try {
            jwt.sign(signer);
        } catch (Exception e) {
            throw new IllegalStateException("Failed to sign service JWT", e);
        }
        return jwt.serialize();
    }
}
