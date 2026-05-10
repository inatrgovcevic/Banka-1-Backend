package com.banka1.banking_service.transfer_service.rabbitmq;

import lombok.Getter;

/**
 * Definiše tipove email poruka i mapira ih na odgovarajuće routing ključeve unutar RabbitMQ-a.
 */
@Getter
public enum EmailType {
    TRANSFER_COMPLETED("transfer.completed"), // Uspešno izvršen prenos
    TRANSFER_FAILED("transfer.failed"); // Neuspešan pokušaj prenosa

    private final String routingKey;

    EmailType(String routingKey) {
        this.routingKey = routingKey;
    }

}
