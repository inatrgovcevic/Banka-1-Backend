package com.banka1.interbank.model.enums;

/**
 * PR_32 Phase 3: status zivotnog ciklusa interbank poruke (po
 * {@link Direction}).
 *
 * <p>INBOUND poruke prelaze direktno u {@link #INBOUND_PROCESSED} cim se cached
 * response upise. OUTBOUND poruke krecu u {@link #PENDING_SEND}, prelaze u
 * {@link #SENT} posle prvog 2xx odgovora, ili u {@link #STUCK} posle 5+
 * neuspesnih retry-a (vidi spec §2.2).
 */
public enum MessageStatus {
    INBOUND_PROCESSED,
    PENDING_SEND,
    SENT,
    STUCK
}
