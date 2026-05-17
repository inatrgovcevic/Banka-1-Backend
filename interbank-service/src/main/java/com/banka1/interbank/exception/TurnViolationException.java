package com.banka1.interbank.exception;

/**
 * PR_32 Phase 10 Task 10.2: pokusaj counter-offer-a (§3.3) ili accept-a
 * (§3.6) kad nije na nama red — protokol forsira striktno smenjivanje
 * {@code lastModifiedBy} izmedju buyer-a i seller-a.
 *
 * <p>Mapira u HTTP 409 Conflict per Tim 2 §6.3 update (KRITICNO — NE 400!)
 * kroz {@link InterbankGlobalExceptionHandler}.
 */
public class TurnViolationException extends RuntimeException {

    public TurnViolationException(String message) {
        super(message);
    }
}
