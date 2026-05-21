package com.banka1.order.entity.enums;

/**
 * Append-only history event types for OTC negotiation changes.
 */
public enum OtcNegotiationEventType {
    CREATED,
    COUNTEROFFERED,
    ACCEPTED,
    DECLINED,
    CANCELLED,
    EXPIRED
}
