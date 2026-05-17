package com.banka1.interbank.model.enums;

/**
 * PR_32 Phase 3: 2PC zivotni ciklus interbank transakcije.
 *
 * <ul>
 *   <li>{@link #PREPARED} — posle uspesnog NEW_TX prepare; sva resource
 *       rezervisana.</li>
 *   <li>{@link #COMMITTED} — posle uspesnog COMMIT_TX.</li>
 *   <li>{@link #ROLLED_BACK} — posle ROLLBACK_TX (eksplicitan ili
 *       interno-trigger).</li>
 *   <li>{@link #FAILED} — terminal greska (npr. commit REST poziv pao posle
 *       max retry-a).</li>
 * </ul>
 */
public enum TxStatus {
    PREPARED,
    COMMITTED,
    ROLLED_BACK,
    FAILED
}
