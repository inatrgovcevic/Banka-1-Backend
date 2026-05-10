package com.banka1.banking_service.account_service.rabbitMQ;

import com.banka1.banking_service.account_service.rabbitMQ.CardEventDto;
import com.banka1.banking_service.account_service.rabbitMQ.EmailDto;
import lombok.RequiredArgsConstructor;
import org.springframework.amqp.rabbit.core.RabbitTemplate;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.stereotype.Component;

/**
 * Klijent za slanje asinkrnih poruka na RabbitMQ message broker.
 * <p>
 * Enkapsulira {@link RabbitTemplate} i konfigurisane vrednosti exchange-a i routing ključa.
 * Koristi se za slanje email notifikacija i card event poruka notifikacijskom servisu
 * i ostalim zainteresovanim servisima preko RabbitMQ exchange-a.
 * <p>
 * Poruke su asinkrone (fire-and-forget), što znači da se ne čeka potvrda dostave
 * pre nego što se odgovori klijentima. Ovo čini account-service responzivnijim jer
 * ne blokira glavni tok na čekanju kompleksnih operacija.
 */
@Component
@RequiredArgsConstructor
public class RabbitClient {

    /** Spring AMQP template koji obavlja stvarno slanje poruka. */
    private final RabbitTemplate rabbitTemplate;

    /**
     * Naziv RabbitMQ exchange-a na koji se sve poruke prosleđuju.
     * Konfigurisano iz {@code rabbitmq.exchange} svojstva.
     */
    @Value("${rabbitmq.exchange}")
    private String exchange;

    /**
     * Šalje email notifikaciju na RabbitMQ exchange koristeći routing ključ iz tipa poruke.
     * <p>
     * Poruka se asinkrno prosleđuje notification-servisu za obaveštavanje korisnika.
     * Routing ključ se određuje na osnovu tipa emaila (npr. CARD_CREATED, LIMIT_CHANGED).
     * <p>
     * Ako slanje ne uspe, {@link org.springframework.amqp.AmqpException} će biti bačen
     * i obrađen u {@code GlobalExceptionHandler}-u.
     *
     * @param dto payload poruke sa email podacima, tipom i podeljeničkim informacijama
     * @throws org.springframework.amqp.AmqpException ako slanje na RabbitMQ ne uspe
     */
    public void sendEmailNotification(EmailDto dto) {
        rabbitTemplate.convertAndSend(exchange, dto.getEmailType().getRoutingKey(), dto);
    }

    /**
     * Šalje event o kartici (kreiranju, izmeni statusa, otkazivanju) na RabbitMQ exchange.
     * <p>
     * Poruka se asinkrno prosleđuje zainteresovanim servisima koji prate
     * životni ciklus kartice. Routing ključ se određuje na osnovu tipa event-a
     * (npr. CARD_CREATED, CARD_BLOCKED, CARD_EXPIRED).
     * <p>
     * Ako slanje ne uspe, {@link org.springframework.amqp.AmqpException} će biti bačen
     * i obrađen u {@code GlobalExceptionHandler}-u.
     *
     * @param dto payload poruke sa informacijama o kartici i tipu event-a
     * @throws org.springframework.amqp.AmqpException ako slanje na RabbitMQ ne uspe
     */
    public void sendCardEvent(CardEventDto dto) {
        rabbitTemplate.convertAndSend(exchange, dto.getEventType().getRoutingKey(), dto);
    }
}
