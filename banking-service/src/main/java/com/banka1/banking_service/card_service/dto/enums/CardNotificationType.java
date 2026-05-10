package com.banka1.banking_service.card_service.dto.enums;

/**
 * Card notification event types and their RabbitMQ routing keys.
 */
public enum CardNotificationType {

    CARD_REQUEST_VERIFICATION("card.request_verification"),
    CARD_REQUEST_SUCCESS("card.request_success"),
    CARD_REQUEST_FAILURE("card.request_failure"),
    CARD_BLOCKED("card.blocked"),
    CARD_UNBLOCKED("card.unblocked"),
    CARD_DEACTIVATED("card.deactivated");

    private final String routingKey;

    CardNotificationType(String routingKey) {
        this.routingKey = routingKey;
    }

    public String getRoutingKey() {
        return routingKey;
    }
}
