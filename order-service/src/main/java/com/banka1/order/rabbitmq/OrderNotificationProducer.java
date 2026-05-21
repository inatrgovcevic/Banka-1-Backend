package com.banka1.order.rabbitmq;

import lombok.RequiredArgsConstructor;
import org.springframework.amqp.rabbit.core.RabbitTemplate;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.stereotype.Component;

/**
 * RabbitMQ producer for order and tax notifications.
 * Publishes messages to the {@code employee.events} topic exchange
 * using the routing keys defined in the Celina 3 specification.
 */
@Component
@RequiredArgsConstructor
public class OrderNotificationProducer {

    private final RabbitTemplate rabbitTemplate;

    @Value("${rabbitmq.exchange}")
    private String exchange;

    /**
     * Publishes a notification that an order has been approved and executed.
     *
     * @param payload the notification payload (serialized to JSON by Jackson)
     */
    public void sendOrderApproved(Object payload) {
        rabbitTemplate.convertAndSend(exchange, "order.approved", payload);
    }

    /**
     * Publishes a notification that an order has been declined.
     *
     * @param payload the notification payload (serialized to JSON by Jackson)
     */
    public void sendOrderDeclined(Object payload) {
        rabbitTemplate.convertAndSend(exchange, "order.declined", payload);
    }

    /**
     * Publishes a notification that tax has been collected from a portfolio transaction.
     *
     * @param payload the notification payload (serialized to JSON by Jackson)
     */
    public void sendTaxCollected(Object payload) {
        rabbitTemplate.convertAndSend(exchange, "tax.collected", payload);
    }

    public void sendOtcCounterofferCreated(Object payload) {
        rabbitTemplate.convertAndSend(exchange, "otc.counteroffer.created", payload);
    }

    public void sendOtcOfferAccepted(Object payload) {
        rabbitTemplate.convertAndSend(exchange, "otc.offer.accepted", payload);
    }

    public void sendOtcOfferDeclined(Object payload) {
        rabbitTemplate.convertAndSend(exchange, "otc.offer.declined", payload);
    }

    public void sendOtcOfferCancelled(Object payload) {
        rabbitTemplate.convertAndSend(exchange, "otc.offer.cancelled", payload);
    }

    public void sendOtcContractExpiring(Object payload) {
        rabbitTemplate.convertAndSend(exchange, "otc.contract.expiring", payload);
    }
}
