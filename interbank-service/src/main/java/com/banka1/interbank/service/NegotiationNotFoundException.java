package com.banka1.interbank.service;

/**
 * Option posting referencira negotiation koji ne postoji u lokalnom
 * {@code interbank_negotiations} mirror-u. Razlikuje se od
 * {@link com.banka1.interbank.protocol.dto.NoVoteReason.Reason#OPTION_NEGOTIATION_NOT_FOUND}
 * po tome sto se ekscepcija baca iz commit-time-a (ne prepare-time-a) ili iz
 * negotiation-management endpoint-a.
 */
public class NegotiationNotFoundException extends InterbankException {

    public NegotiationNotFoundException(String message) {
        super(message);
    }
}
