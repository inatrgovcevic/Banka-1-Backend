package com.banka1.order.rabbitmq;

import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;
import org.junit.jupiter.api.extension.ExtendWith;
import org.springframework.test.util.ReflectionTestUtils;

import java.util.Map;

import static org.mockito.Mockito.verify;

@ExtendWith(MockitoExtension.class)
class OrderNotificationProducerTest {

    @Mock
    private org.springframework.amqp.rabbit.core.RabbitTemplate rabbitTemplate;

    private OrderNotificationProducer producer;

    @BeforeEach
    void setUp() {
        producer = new OrderNotificationProducer(rabbitTemplate);
        ReflectionTestUtils.setField(producer, "exchange", "employee.events");
    }

    @Test
    void sendsExistingAndOtcNotificationsWithExpectedRoutingKeys() {
        Map<String, Object> payload = Map.of("foo", "bar");

        producer.sendOrderApproved(payload);
        producer.sendOrderDeclined(payload);
        producer.sendTaxCollected(payload);
        producer.sendOtcCounterofferCreated(payload);
        producer.sendOtcOfferAccepted(payload);
        producer.sendOtcOfferDeclined(payload);
        producer.sendOtcOfferCancelled(payload);
        producer.sendOtcContractExpiring(payload);

        verify(rabbitTemplate).convertAndSend("employee.events", "order.approved", payload);
        verify(rabbitTemplate).convertAndSend("employee.events", "order.declined", payload);
        verify(rabbitTemplate).convertAndSend("employee.events", "tax.collected", payload);
        verify(rabbitTemplate).convertAndSend("employee.events", "otc.counteroffer.created", payload);
        verify(rabbitTemplate).convertAndSend("employee.events", "otc.offer.accepted", payload);
        verify(rabbitTemplate).convertAndSend("employee.events", "otc.offer.declined", payload);
        verify(rabbitTemplate).convertAndSend("employee.events", "otc.offer.cancelled", payload);
        verify(rabbitTemplate).convertAndSend("employee.events", "otc.contract.expiring", payload);
    }
}
