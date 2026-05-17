package com.banka1.interbank.exception;

import com.banka1.interbank.service.InterbankException;
import jakarta.validation.ConstraintViolationException;
import java.util.Map;
import lombok.extern.slf4j.Slf4j;
import org.springframework.http.ResponseEntity;
import org.springframework.http.converter.HttpMessageNotReadableException;
import org.springframework.security.access.AccessDeniedException;
import org.springframework.security.authorization.AuthorizationDeniedException;
import org.springframework.web.bind.MethodArgumentNotValidException;
import org.springframework.web.bind.annotation.ExceptionHandler;
import org.springframework.web.bind.annotation.RestControllerAdvice;

/**
 * PR_32 Phase 10 Task 10.2: centralni handler za OTC §3 rute (per Tim 2 §6.3
 * response codes).
 *
 * <p>Mapping:
 * <ul>
 *   <li>{@link NegotiationNotFoundException} → 404</li>
 *   <li>{@link TurnViolationException} +
 *       {@link NegotiationClosedException} → <strong>409 Conflict</strong>
 *       (KRITICNO — NE 400 per Tim 2 §6.3 update)</li>
 *   <li>{@link InvalidNegotiationException} + Bean Validation greske +
 *       malformed JSON + IllegalArgumentException → 400 Bad Request</li>
 *   <li>{@link AccessDeniedException} +
 *       {@link AuthorizationDeniedException} → 403 (HOTFIX 36 iz PR_31)</li>
 *   <li>{@link InterbankException} (2PC fail) → 500</li>
 * </ul>
 *
 * <p>NE hvata generic {@code Exception.class} — to ostavlja Spring-ovom
 * default-u (500 ali bez body-ja). Razlog: ne zelimo da slucajno "progutamo"
 * Spring-ove protokolarne ekscepcije (nesto poput HandlerMappingException) i
 * vratimo nasao body koji ce zbuniti partner banku.
 */
@RestControllerAdvice
@Slf4j
public class InterbankGlobalExceptionHandler {

    @ExceptionHandler(NegotiationNotFoundException.class)
    public ResponseEntity<Map<String, String>> notFound(NegotiationNotFoundException e) {
        log.debug("Negotiation not found: {}", e.getMessage());
        return ResponseEntity.status(404).body(Map.of("error", e.getMessage()));
    }

    /**
     * KRITICNO — Tim 2 §6.3 update: turn violation i closed negotiation MORAJU
     * vracati 409 Conflict, NE 400. Ranija verzija spec-a je rekla 400; Tim 2
     * je posle protokol-mediation sesije izmenio na 409.
     */
    @ExceptionHandler({TurnViolationException.class, NegotiationClosedException.class})
    public ResponseEntity<Map<String, String>> conflict(RuntimeException e) {
        log.debug("Conflict: {}", e.getMessage());
        return ResponseEntity.status(409).body(Map.of("error", e.getMessage()));
    }

    @ExceptionHandler({
            InvalidNegotiationException.class,
            MethodArgumentNotValidException.class,
            HttpMessageNotReadableException.class,
            ConstraintViolationException.class,
            IllegalArgumentException.class
    })
    public ResponseEntity<Map<String, String>> badRequest(Exception e) {
        log.debug("Bad request: {}", e.getMessage());
        String msg = e.getMessage() == null ? "Invalid request" : e.getMessage();
        return ResponseEntity.status(400).body(Map.of("error", msg));
    }

    @ExceptionHandler({AccessDeniedException.class, AuthorizationDeniedException.class})
    public ResponseEntity<Void> forbidden() {
        return ResponseEntity.status(403).build();
    }

    @ExceptionHandler(InterbankException.class)
    public ResponseEntity<Map<String, String>> interbankFail(InterbankException e) {
        log.error("Interbank operation failed", e);
        return ResponseEntity.status(500).body(Map.of("error", e.getMessage()));
    }
}
