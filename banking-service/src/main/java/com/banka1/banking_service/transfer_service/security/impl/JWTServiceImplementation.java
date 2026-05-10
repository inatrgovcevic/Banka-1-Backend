package com.banka1.banking_service.transfer_service.security.impl;

import com.banka1.banking_service.transfer_service.security.JWTService;
import com.nimbusds.jose.JWSAlgorithm;
import com.nimbusds.jose.JWSHeader;
import com.nimbusds.jose.JWSSigner;
import com.nimbusds.jose.KeyLengthException;
import com.nimbusds.jose.crypto.MACSigner;
import com.nimbusds.jwt.JWTClaimsSet;
import com.nimbusds.jwt.SignedJWT;
import lombok.Getter;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.stereotype.Service;

import java.util.ArrayList;
import java.util.Date;
import java.util.List;

/**
 * Implementacija {@link JWTService} interfejsa koja koristi Nimbus-JOSE-JWT biblioteku.
 * Kreira tokene potpisane HMAC-SHA256 algoritmom koristeći deljenu tajnu (shared secret).
 */
@Service
@Getter
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

    /**
     * Konstruktor koji inicijalizuje signer objekt koristeći tajni ključ.
     * @param secret tajna ucitana iz konfiguracije (jwt.secret)
     * @throws KeyLengthException ako je tajna prekratka za HS256 standard
     */
    public JWTServiceImplementation(@Value("${jwt.secret}") String secret) throws KeyLengthException {
        this.signer = new MACSigner(secret);
    }

    /**
     * Generiše novi JWT token sa claim-ovima specifičnim za transfer-servis.
     * Token uključuje ulogu "SERVICE", issuer-a i definisano vreme trajanja.
     * @return potpisan i serijalizovan JWT
     */
    @Override
    public String generateJwtToken() {
        // Sistemski token obično nema specifične permisije, već se oslanja na "SERVICE" ulogu
        List<String> permissions = new ArrayList<>();

        JWTClaimsSet claims = new JWTClaimsSet.Builder()
                .subject("transfer-service") // Identifikator tvog servisa
                .issuer(issuer)
                .claim(roleClaim, "SERVICE") // Ključno: uloga je SERVICE
                .claim(permissionClaim, permissions)
                .expirationTime(new Date(System.currentTimeMillis() + expirationTime))
                .build();

        JWSHeader header = new JWSHeader(JWSAlgorithm.HS256);
        SignedJWT jwt = new SignedJWT(header, claims);
        try {
            jwt.sign(signer);
        } catch (Exception e) {
            throw new IllegalStateException("Greška pri potpisivanju sistemskog JWT-a", e);
        }
        return jwt.serialize();
    }
}
