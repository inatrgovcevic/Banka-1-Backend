package com.banka1.interbank.exception;

/**
 * PR_32 Phase 10 Task 10.2: pokusaj counter-offer-a / accept-a na pregovor
 * koji je vec zatvoren (isOngoing=false) ili kome je settlement_date prosao.
 *
 * <p>Mapira u HTTP 409 Conflict per Tim 2 §6.3 kroz
 * {@link InterbankGlobalExceptionHandler}.
 */
public class NegotiationClosedException extends RuntimeException {

    public NegotiationClosedException(String message) {
        super(message);
    }
}
