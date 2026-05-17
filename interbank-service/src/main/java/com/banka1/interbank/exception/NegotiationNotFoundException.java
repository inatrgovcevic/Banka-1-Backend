package com.banka1.interbank.exception;

/**
 * PR_32 Phase 10 Task 10.2: bacanje iz OtcNegotiationService kad se trazena
 * negotiation ne nalazi u {@code interbank_negotiations} mirror-u. Mapira u
 * HTTP 404 Not Found (per Tim 2 §6.3) kroz
 * {@link InterbankGlobalExceptionHandler}.
 *
 * <p>NAPOMENA: Postoji istoimena klasa u paketu
 * {@code com.banka1.interbank.service} koja extend-uje
 * {@link com.banka1.interbank.service.InterbankException} i koristi se u 2PC
 * (commit-time) flow-u. Ova klasa je name-spaced u {@code exception/} paketu i
 * koristi se SAMO iz CRUD endpoint-a /negotiations (§3.3-3.6) gde mapping u
 * 404 mora da bude direktan.
 */
public class NegotiationNotFoundException extends RuntimeException {

    public NegotiationNotFoundException(String message) {
        super(message);
    }
}
