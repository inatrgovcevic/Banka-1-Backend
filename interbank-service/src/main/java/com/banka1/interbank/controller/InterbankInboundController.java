package com.banka1.interbank.controller;

import com.banka1.interbank.auth.InterbankAuthenticationToken;
import com.banka1.interbank.config.InterbankProperties;
import com.banka1.interbank.model.InterbankMessageEntity;
import com.banka1.interbank.model.enums.Direction;
import com.banka1.interbank.protocol.dto.InterbankMessagePayload;
import com.banka1.interbank.service.InterbankMessageService;
import com.fasterxml.jackson.databind.ObjectMapper;
import jakarta.validation.Valid;
import java.util.Map;
import java.util.Optional;
import lombok.RequiredArgsConstructor;
import lombok.extern.slf4j.Slf4j;
import org.springframework.http.ResponseEntity;
import org.springframework.security.core.context.SecurityContextHolder;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.RequestBody;
import org.springframework.web.bind.annotation.RestController;

/**
 * PR_32 Phase 7 Task 7.2: HTTP shell za INBOUND inter-bank pozive.
 *
 * <p>Tok obrade:
 * <ol>
 *   <li>{@link com.banka1.interbank.auth.InterbankAuthFilter} (Phase 4) validuje X-Api-Key
 *       header pre dolaska do ovog controller-a i postavlja
 *       {@link InterbankAuthenticationToken} u {@link SecurityContextHolder}.</li>
 *   <li>Controller cita partner routing iz auth principal-a i poredi sa
 *       {@code idempotenceKey.routingNumber} u payload-u — mismatch znaci da partner pokusava
 *       da impersonira drugu banku, vraca se 400.</li>
 *   <li>Idempotency cache lookup po (INBOUND, senderRouting, locallyGeneratedKey). Hit →
 *       cached HTTP status + body bez ponovne obrade.</li>
 *   <li>Miss → delegira na {@link InboundDispatcher} koji ima public {@code @Transactional}
 *       metode (Spring AOP ne radi za internal poziv u istoj klasi, zato je dispatcher
 *       odvojen).</li>
 * </ol>
 */
@RestController
@RequiredArgsConstructor
@Slf4j
public class InterbankInboundController {

    private final InboundDispatcher dispatcher;
    private final InterbankMessageService messageService;
    private final ObjectMapper mapper;

    /**
     * Glavni INBOUND endpoint per Tim 2 §6.1. Validacija payload-a se vrsi kroz
     * {@code @Valid} (NotNull/NotBlank/Size na DTO record-ima).
     *
     * @param msg validirani message envelope
     * @return ResponseEntity sa odgovarajucim status code-om i (kad je primenljivo) body-jem;
     *         cache hit slucajevi vracaju identican payload kao i prvi response
     */
    @PostMapping("/interbank")
    public ResponseEntity<?> inbound(@Valid @RequestBody InterbankMessagePayload msg) {
        var auth = SecurityContextHolder.getContext().getAuthentication();
        if (!(auth instanceof InterbankAuthenticationToken interbankAuth)) {
            log.warn("Inbound /interbank pozvan bez InterbankAuthenticationToken-a u SecurityContext-u");
            return ResponseEntity.status(401).build();
        }
        InterbankProperties.Partner partner = (InterbankProperties.Partner) interbankAuth.getPrincipal();
        int senderRouting = partner.getRoutingNumber();

        // Routing-number mismatch — partner pokusava da impersonira drugu banku.
        if (senderRouting != msg.idempotenceKey().routingNumber()) {
            log.warn("Routing mismatch: X-Api-Key sender={} but idempotenceKey.routingNumber={}",
                    senderRouting, msg.idempotenceKey().routingNumber());
            return ResponseEntity.badRequest().body(Map.of(
                    "error", "idempotenceKey.routingNumber mismatches X-Api-Key sender"));
        }

        // Idempotency cache lookup.
        Optional<InterbankMessageEntity> cached = messageService.findCached(
                Direction.INBOUND, senderRouting, msg.idempotenceKey().locallyGeneratedKey());
        if (cached.isPresent()) {
            InterbankMessageEntity c = cached.get();
            log.info("Idempotency cache hit: sender={} key={} httpStatus={}",
                    senderRouting, msg.idempotenceKey().locallyGeneratedKey(), c.getHttpStatus());
            int status = c.getHttpStatus() == null ? 200 : c.getHttpStatus();
            if (status == 204) {
                return ResponseEntity.noContent().build();
            }
            return ResponseEntity.status(status).body(parseBody(c.getResponseBody()));
        }

        return switch (msg.messageType()) {
            case NEW_TX      -> dispatcher.handleNewTx(msg);
            case COMMIT_TX   -> dispatcher.handleCommitTx(msg);
            case ROLLBACK_TX -> dispatcher.handleRollbackTx(msg);
        };
    }

    /** Vrati cached response body deserializovan u JSON Object — fallback na raw string. */
    private Object parseBody(String body) {
        if (body == null) {
            return null;
        }
        try {
            return mapper.readValue(body, Object.class);
        } catch (Exception e) {
            return body;
        }
    }
}
