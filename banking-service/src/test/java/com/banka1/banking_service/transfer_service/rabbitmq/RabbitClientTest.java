package com.banka1.banking_service.transfer_service.rabbitmq;

import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.mockito.Mock;
import org.mockito.MockitoAnnotations;
import org.springframework.amqp.rabbit.core.RabbitTemplate;
import org.springframework.test.util.ReflectionTestUtils;

import static org.mockito.ArgumentMatchers.eq;
import static org.mockito.Mockito.verify;

class RabbitClientTest {

    private RabbitClient rabbitClient;

    @Mock
    private RabbitTemplate rabbitTemplate;

    @BeforeEach
    void setUp() {
        MockitoAnnotations.openMocks(this);
        rabbitClient = new RabbitClient(rabbitTemplate);
        // Ručno postavljamo @Value polje
        ReflectionTestUtils.setField(rabbitClient, "exchange", "test-exchange");
    }

    @Test
    void sendEmailNotification_ShouldCallRabbitTemplate() {
        EmailDto dto = new EmailDto("Ime", "test@test.com", EmailType.TRANSFER_COMPLETED, "Poruka");

        rabbitClient.sendEmailNotification(dto);

        // Proveravamo da li je rabbitTemplate pozvan sa ispravnim exchange-om i routing key-om
        verify(rabbitTemplate).convertAndSend(
                eq("test-exchange"),
                eq(EmailType.TRANSFER_COMPLETED.getRoutingKey()),
                eq(dto)
        );
    }
}