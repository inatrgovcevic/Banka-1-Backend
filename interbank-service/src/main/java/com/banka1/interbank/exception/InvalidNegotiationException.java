package com.banka1.interbank.exception;

/**
 * PR_32 Phase 10 Task 10.2: payload validacioni problem koji nije pokriven
 * Bean Validation-om (npr. settlementDate u proslosti, routing-number
 * mismatch sa X-Api-Key sender-om, negativan amount/price).
 *
 * <p>Mapira u HTTP 400 Bad Request per Tim 2 §6.3 kroz
 * {@link InterbankGlobalExceptionHandler}.
 */
public class InvalidNegotiationException extends RuntimeException {

    public InvalidNegotiationException(String message) {
        super(message);
    }
}
