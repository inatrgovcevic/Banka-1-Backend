package com.banka1.banking_service.card_service.integration;

import com.banka1.banking_service.card_service.rabbitMQ.RabbitConfig;
import org.junit.jupiter.api.Test;
import org.springframework.amqp.core.Binding;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.beans.factory.annotation.Qualifier;
import org.springframework.boot.test.context.SpringBootTest;
import org.springframework.test.context.TestPropertySource;

import static org.junit.jupiter.api.Assertions.assertEquals;

@SpringBootTest(
        classes = RabbitConfig.class,
        webEnvironment = SpringBootTest.WebEnvironment.NONE
)
@TestPropertySource(properties = {
        "spring.rabbitmq.host=localhost",
        "spring.rabbitmq.port=5672",
        "spring.rabbitmq.username=guest",
        "spring.rabbitmq.password=guest",
        "notification.rabbit.exchange=employee.events",
        "card.rabbit.auto.exchange=employee.events",
        "card.rabbit.auto.queue=card-service-auto-card-queue",
        "card.rabbit.auto.routing-key=card.create"
})
class RabbitBindingsConfigIntegrationTest {

    @Autowired
    @Qualifier("card_automaticCardCreationBinding")
    private Binding automaticCardCreationBinding;

    @Test
    void automaticCardCreationBindingUsesCardCreateRoutingKey() {
        assertEquals("card.create", automaticCardCreationBinding.getRoutingKey());
    }
}
