package com.banka1.interbank.model.enums;

/**
 * PR_32 Phase 3: status interbank opcionog ugovora (vidi
 * {@code interbank_contracts}).
 *
 * <ul>
 *   <li>{@link #PENDING_PREMIUM} — kontrakt kreiran, ali premium 2PC jos nije
 *       commit-ovan. Resava KRIT bug #2 (option contract kreiran PRE premium
 *       SAGA u intra-bank flow-u).</li>
 *   <li>{@link #ACTIVE} — premium prebacen, ugovor zive i moze se exercise-ovati.</li>
 *   <li>{@link #EXERCISED} — kupac iskoristio opciju (call/put).</li>
 *   <li>{@link #EXPIRED} — settlement_date prosao bez exercise-a (auto-flip
 *       kroz scheduled job).</li>
 *   <li>{@link #CANCELED} — eksplicitan rollback premium SAGA-e ili admin
 *       intervencija.</li>
 * </ul>
 */
public enum NegotiationContractStatus {
    PENDING_PREMIUM,
    ACTIVE,
    EXERCISED,
    EXPIRED,
    CANCELED
}
