package com.banka1.banking_service.transfer_service.rabbitmq;

import com.banka1.banking_service.transfer_service.rabbitmq.EmailDto;
import com.banka1.banking_service.transfer_service.rabbitmq.EmailType;
import lombok.RequiredArgsConstructor;
import org.springframework.amqp.rabbit.core.RabbitTemplate;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.stereotype.Component;

/**
 * Klijent zadužen za slanje poruka u RabbitMQ exchange.
 */
@Component
@RequiredArgsConstructor
public class RabbitClient {

    private final RabbitTemplate rabbitTemplate;

    @Value("${rabbitmq.exchange}")
    private String exchange;
    /**
     * Šalje email notifikaciju na definisani exchange koristeći routing key iz {@link EmailType}.
     * @param dto podaci o notifikaciji
     */
    public void sendEmailNotification(EmailDto dto) {
        // Koristimo routing key definisan u enum-u (npr. "transfer.completed")
        rabbitTemplate.convertAndSend(exchange, dto.getEmailType().getRoutingKey(), dto);
    }
}