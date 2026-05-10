package com.banka1.banking_service.card_service.rabbitMQ;

import com.fasterxml.jackson.annotation.JsonIgnoreProperties;
import lombok.AllArgsConstructor;
import lombok.Getter;
import lombok.NoArgsConstructor;
import lombok.Setter;

/**
 * RabbitMQ payload published by account-service when a new account should
 * trigger automatic card creation.
 */
@JsonIgnoreProperties(ignoreUnknown = true)
@Getter
@Setter
@NoArgsConstructor
@AllArgsConstructor
public class CardCreationEventDto {

    private Long clientId;
    private String accountNumber;
    private String eventType;
}
