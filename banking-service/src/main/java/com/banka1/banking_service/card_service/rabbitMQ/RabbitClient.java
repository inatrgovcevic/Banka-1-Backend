package com.banka1.banking_service.card_service.rabbitMQ;

import com.banka1.banking_service.card_service.dto.card_management.internal.CardNotificationDto;
import com.banka1.banking_service.card_service.dto.enums.CardNotificationType;
import lombok.RequiredArgsConstructor;
import org.springframework.amqp.rabbit.core.RabbitTemplate;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.stereotype.Component;

/**
 * Client for publishing messages to RabbitMQ.
 * Encapsulates {@link RabbitTemplate} and the configured exchange name,
 * so that services do not depend on RabbitMQ infrastructure directly.
 *
 */
@Component
@RequiredArgsConstructor
public class RabbitClient {

    /** Spring AMQP template that performs the actual message publishing. */
    private final RabbitTemplate rabbitTemplate;

    /** Name of the RabbitMQ exchange to which messages are published. */
    @Value("${notification.rabbit.exchange}")
    private String exchange;

    /**
     * Publishes a card notification event to the configured exchange.
     *
     * @param notificationType typed routing key descriptor
     * @param payload message payload to publish
     */
    public void sendCardNotification(CardNotificationType notificationType, CardNotificationDto payload) {
        rabbitTemplate.convertAndSend(exchange, notificationType.getRoutingKey(), payload);
    }
}
