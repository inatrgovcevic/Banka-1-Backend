package com.banka1.interbank.model.enums;

/**
 * PR_32 Phase 3: smer interbank poruke.
 *
 * <ul>
 *   <li>{@link #INBOUND} — primljena od partnerske banke (mi smo callee).</li>
 *   <li>{@link #OUTBOUND} — poslata partnerskoj banci (mi smo caller).</li>
 * </ul>
 */
public enum Direction {
    INBOUND,
    OUTBOUND
}
