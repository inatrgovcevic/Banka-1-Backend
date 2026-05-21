package com.banka1.order.entity.enums;

/**
 * Lifecycle status of one OTC negotiation or accepted OTC contract.
 */
public enum OtcNegotiationStatus {
    OPEN,
    COUNTERED,
    ACCEPTED,
    DECLINED,
    CANCELLED,
    EXPIRED
}
