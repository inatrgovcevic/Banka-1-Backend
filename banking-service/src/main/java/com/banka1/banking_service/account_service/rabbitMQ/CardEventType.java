package com.banka1.banking_service.account_service.rabbitMQ;

public enum CardEventType {

    CARD_CREATE("card.create"),
    CARD_DEACTIVATE("card.deactivate");

    private final String routingKey;

    CardEventType(String routingKey) {
        this.routingKey = routingKey;
    }

    public String getRoutingKey() {
        return routingKey;
    }
}
