package com.banka1.banking_service.card_service.dto.card_management.internal;

import com.fasterxml.jackson.annotation.JsonInclude;
import lombok.AllArgsConstructor;
import lombok.Getter;
import lombok.NoArgsConstructor;
import lombok.Setter;

import java.util.Map;

/**
 * RabbitMQ payload compatible with notification-service NotificationRequest contract.
 */
@Getter
@Setter
@NoArgsConstructor
@AllArgsConstructor
@JsonInclude(JsonInclude.Include.NON_NULL)
public class CardNotificationDto {

    private String username;
    private String userEmail;
    private Map<String, String> templateVariables;
}
