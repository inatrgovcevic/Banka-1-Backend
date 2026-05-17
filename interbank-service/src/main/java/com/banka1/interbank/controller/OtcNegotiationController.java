package com.banka1.interbank.controller;

import com.banka1.interbank.auth.InterbankAuthenticationToken;
import com.banka1.interbank.config.InterbankProperties;
import com.banka1.interbank.otc.dto.OtcNegotiationDto;
import com.banka1.interbank.otc.dto.OtcOfferDto;
import com.banka1.interbank.protocol.dto.ForeignBankId;
import com.banka1.interbank.service.OtcNegotiationService;
import jakarta.validation.Valid;
import lombok.RequiredArgsConstructor;
import lombok.extern.slf4j.Slf4j;
import org.springframework.http.ResponseEntity;
import org.springframework.security.core.context.SecurityContextHolder;
import org.springframework.web.bind.annotation.DeleteMapping;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.PathVariable;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.PutMapping;
import org.springframework.web.bind.annotation.RequestBody;
import org.springframework.web.bind.annotation.RestController;

/**
 * PR_32 Phase 10 Task 10.5: OTC §3 negotiation endpoints (Tim 2 §3.2-3.6).
 *
 * <p><strong>KRITICNO — Tim 2 §6.3:</strong>
 * <ul>
 *   <li>PUT counter-offer: 204 / 404 / <strong>409 (turn or closed)</strong> / 400 (malformed)</li>
 *   <li>GET accept: 204 (posle 2PC commit) / 404 / 409 / 5xx</li>
 * </ul>
 *
 * <p>Sve metode dohvataju sender's routing iz X-Api-Key autentifikacije
 * kroz {@link InterbankAuthenticationToken} u SecurityContext-u (postavljen
 * iz {@link com.banka1.interbank.auth.InterbankAuthFilter}).
 */
@RestController
@RequiredArgsConstructor
@Slf4j
public class OtcNegotiationController {

    private final OtcNegotiationService service;

    /**
     * §3.2 POST /negotiations — kreiraj novi pregovor (buyer iz druge banke).
     * Vraca {@link ForeignBankId} sa nasim routing brojem i generisanim ID-jem.
     */
    @PostMapping("/negotiations")
    public ResponseEntity<ForeignBankId> create(@Valid @RequestBody OtcOfferDto offer) {
        int senderRouting = requireSenderRouting();
        ForeignBankId id = service.createNegotiation(offer, senderRouting);
        return ResponseEntity.ok(id);
    }

    /**
     * §3.4 GET /negotiations/{rn}/{id} — vrati trenutno stanje pregovora.
     */
    @GetMapping("/negotiations/{rn}/{id}")
    public OtcNegotiationDto get(@PathVariable int rn, @PathVariable String id) {
        return service.getNegotiation(rn, id);
    }

    /**
     * §3.3 PUT /negotiations/{rn}/{id} — counter-offer.
     *
     * <p>Tim 2 §6.3:
     * <ul>
     *   <li>204 — happy path</li>
     *   <li>409 — turn violation ILI negotiation zatvoren</li>
     *   <li>400 — malformed (routing mismatch, past settlement, negativan iznos)</li>
     *   <li>404 — negotiation ne postoji</li>
     * </ul>
     */
    @PutMapping("/negotiations/{rn}/{id}")
    public ResponseEntity<Void> update(@PathVariable int rn,
                                       @PathVariable String id,
                                       @Valid @RequestBody OtcOfferDto offer) {
        int senderRouting = requireSenderRouting();
        service.updateCounter(rn, id, offer, senderRouting);
        return ResponseEntity.noContent().build();
    }

    /**
     * §3.5 DELETE /negotiations/{rn}/{id} — close pregovor (idempotent).
     */
    @DeleteMapping("/negotiations/{rn}/{id}")
    public ResponseEntity<Void> delete(@PathVariable int rn, @PathVariable String id) {
        service.delete(rn, id);
        return ResponseEntity.noContent().build();
    }

    /**
     * §3.6 GET /negotiations/{rn}/{id}/accept — sinhronizovan 2PC accept.
     *
     * <p><strong>Tim 2 §6.6:</strong> konekcija moze drzati do 60s; 204 se
     * vraca tek kad je COMMIT_TX poslato partneru. 5xx ako 2PC fail-uje
     * (kroz {@link com.banka1.interbank.service.InterbankException} u
     * GlobalExceptionHandler-u).
     */
    @GetMapping("/negotiations/{rn}/{id}/accept")
    public ResponseEntity<Void> accept(@PathVariable int rn, @PathVariable String id) {
        int senderRouting = requireSenderRouting();
        service.acceptNegotiation(rn, id, senderRouting);
        return ResponseEntity.noContent().build();
    }

    /**
     * Ekstraktuj routing number iz X-Api-Key autentifikacije.
     * Vraca 401 ako Security context nije InterbankAuthenticationToken.
     */
    private int requireSenderRouting() {
        var auth = SecurityContextHolder.getContext().getAuthentication();
        if (!(auth instanceof InterbankAuthenticationToken interbankAuth)) {
            throw new IllegalStateException(
                    "Missing InterbankAuthenticationToken — InterbankAuthFilter prerequisite");
        }
        InterbankProperties.Partner partner =
                (InterbankProperties.Partner) interbankAuth.getPrincipal();
        return partner.getRoutingNumber();
    }
}
