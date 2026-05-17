package com.banka1.interbank.otc.wrapper;

import com.banka1.interbank.otc.wrapper.dto.OutboundCounterOfferRequest;
import com.banka1.interbank.otc.wrapper.dto.OutboundCreateNegotiationRequest;
import com.banka1.interbank.otc.wrapper.dto.OutboundNegotiationResponse;
import jakarta.validation.Valid;
import java.util.List;
import lombok.RequiredArgsConstructor;
import lombok.extern.slf4j.Slf4j;
import org.springframework.http.ResponseEntity;
import org.springframework.security.access.prepost.PreAuthorize;
import org.springframework.security.core.annotation.AuthenticationPrincipal;
import org.springframework.security.oauth2.jwt.Jwt;
import org.springframework.web.bind.annotation.DeleteMapping;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.PathVariable;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.PutMapping;
import org.springframework.web.bind.annotation.RequestBody;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;

/**
 * PR_33 Phase A: FE-facing wrapper za inter-bank OTC pregovore.
 *
 * <p>Analogno Tim 2's {@code InterbankOtcWrapperController}. Path namespace
 * {@code /api/interbank/otc/*} odvaja FE-pozive od inter-bank protokol ruta
 * (X-Api-Key auth na {@code /interbank}, {@code /negotiations}, {@code /public-stock}).
 * Ove rute koriste JWT auth iz security-lib chain-a (@Order 2).
 *
 * <p>Auth permissions per Celina 4 spec:
 * <ul>
 *   <li>POST/PUT/DELETE/accept: {@code hasAuthority('OTC_TRADE')} (klijent
 *       koji ima OTC dozvolu).</li>
 *   <li>GET list/get one: {@code hasAuthority('OTC_TRADE')} ili admin/supervisor.</li>
 * </ul>
 *
 * <p>JWT principal expose-ovan kroz {@code @AuthenticationPrincipal Jwt} —
 * {@code jwt.getClaim("id")} vraca {@code Long} (numeric user id iz user-service-a).
 *
 * <p>Sve operacije propagiraju partner-ov HTTP status code nazad FE-u:
 * 204/400/404/409/500. Local mirror se update-uje SAMO ako je partner 2xx.
 */
@RestController
@RequestMapping("/api/interbank/otc")
@RequiredArgsConstructor
@Slf4j
public class InterbankOtcOutboundController {

    private final InterbankOtcOutboundService service;

    /**
     * POST /api/interbank/otc/negotiations — kreiraj outbound pregovor sa
     * drugom bankom. Mi smo buyer-bank.
     */
    @PostMapping("/negotiations")
    @PreAuthorize("hasAuthority('OTC_TRADE') or hasRole('ADMIN') or hasRole('SUPERVISOR')")
    public ResponseEntity<OutboundNegotiationResponse> create(
            @AuthenticationPrincipal Jwt jwt,
            @Valid @RequestBody OutboundCreateNegotiationRequest req) {
        Long principalId = extractPrincipalId(jwt);
        Long buyerId = req.buyerLocalUserId() != null ? req.buyerLocalUserId() : principalId;
        OutboundNegotiationResponse resp = service.createOutbound(req, buyerId);
        return ResponseEntity.ok(resp);
    }

    /**
     * GET /api/interbank/otc/negotiations — list pregovore u kojima ucestvuje user.
     *
     * <p>Admin i SUPERVISOR vide SVE inter-bank pregovore (cross-tenant view).
     * Klijent vidi samo svoje (buyer ili seller scope).
     */
    @GetMapping("/negotiations")
    @PreAuthorize("hasAuthority('OTC_TRADE') or hasRole('ADMIN') or hasRole('SUPERVISOR')")
    public ResponseEntity<List<OutboundNegotiationResponse>> list(@AuthenticationPrincipal Jwt jwt) {
        Long principalId = extractPrincipalId(jwt);
        boolean includeAll = hasAdminOrSupervisorRole(jwt);
        return ResponseEntity.ok(service.listForUser(principalId, includeAll));
    }

    /**
     * GET /api/interbank/otc/negotiations/{id} — fetch state pregovora.
     */
    @GetMapping("/negotiations/{id}")
    @PreAuthorize("hasAuthority('OTC_TRADE') or hasRole('ADMIN') or hasRole('SUPERVISOR')")
    public ResponseEntity<OutboundNegotiationResponse> get(@PathVariable String id) {
        return ResponseEntity.ok(service.getOne(id));
    }

    /**
     * PUT /api/interbank/otc/negotiations/{id}/counter — counter-offer ka partner-u.
     *
     * <p>Vraca partner-ov status code (204/400/404/409). Tim 2 §6.3 garantuje
     * da je 409 = turn violation ili closed; 400 = malformed payload;
     * 404 = negotiation ne postoji.
     */
    @PutMapping("/negotiations/{id}/counter")
    @PreAuthorize("hasAuthority('OTC_TRADE') or hasRole('ADMIN') or hasRole('SUPERVISOR')")
    public ResponseEntity<OutboundNegotiationResponse> counter(
            @AuthenticationPrincipal Jwt jwt,
            @PathVariable String id,
            @Valid @RequestBody OutboundCounterOfferRequest req) {
        Long principalId = extractPrincipalId(jwt);
        return service.counterOutbound(id, req, principalId);
    }

    /**
     * POST /api/interbank/otc/negotiations/{id}/accept — accept current offer.
     *
     * <p>Pokrece outbound GET /negotiations/{rn}/{id}/accept ka partner-u
     * (seller-bank). Partner pokrece 2PC (mi smo participant). Po Tim 2 §6.6
     * konekcija moze drzati do 60s.
     */
    @PostMapping("/negotiations/{id}/accept")
    @PreAuthorize("hasAuthority('OTC_TRADE') or hasRole('ADMIN') or hasRole('SUPERVISOR')")
    public ResponseEntity<Void> accept(@PathVariable String id) {
        return service.acceptOutbound(id);
    }

    /**
     * GET /api/interbank/otc/public-stock?bankCode={routingNumber} —
     * FE discovery view: dohvati javne akcije iz partner banke.
     *
     * <p>Naš wrapper proxify-uje na partner-ov {@code GET /public-stock}
     * (X-Api-Key auth). Default bankCode = 222 (Banka 2) ako nije zadat.
     * Ako partner nije dostupan ili je odgovor 4xx/5xx, vracamo praznu listu
     * (UI bude prazna tabela, ne error).
     */
    @GetMapping("/public-stock")
    @PreAuthorize("hasAuthority('OTC_TRADE') or hasRole('ADMIN') or hasRole('SUPERVISOR')")
    public ResponseEntity<List<com.banka1.interbank.otc.dto.PublicStockEntryDto>> partnerPublicStock(
            @org.springframework.web.bind.annotation.RequestParam(name = "bankCode", required = false, defaultValue = "222")
            int bankCode) {
        return ResponseEntity.ok(service.fetchPartnerPublicStock(bankCode));
    }

    /**
     * DELETE /api/interbank/otc/negotiations/{id} — close pregovor.
     */
    @DeleteMapping("/negotiations/{id}")
    @PreAuthorize("hasAuthority('OTC_TRADE') or hasRole('ADMIN') or hasRole('SUPERVISOR')")
    public ResponseEntity<Void> delete(@PathVariable String id) {
        return service.deleteOutbound(id);
    }

    /**
     * Ekstraktuj numeric user id iz JWT {@code id} claim-a. Vraca null ako
     * claim nedostaje (admin tokeni iz service-to-service auth-a npr.).
     */
    private Long extractPrincipalId(Jwt jwt) {
        if (jwt == null) {
            return null;
        }
        Object claim = jwt.getClaim("id");
        if (claim instanceof Number n) {
            return n.longValue();
        }
        if (claim instanceof String s) {
            try {
                return Long.parseLong(s);
            } catch (NumberFormatException e) {
                return null;
            }
        }
        return null;
    }

    private boolean hasAdminOrSupervisorRole(Jwt jwt) {
        if (jwt == null) {
            return false;
        }
        List<String> roles = jwt.getClaimAsStringList("roles");
        if (roles == null) {
            return false;
        }
        return roles.contains("ADMIN") || roles.contains("SUPERVISOR");
    }
}
