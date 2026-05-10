package com.banka1.banking_service.card_service.rabbitMQ;

import org.springframework.amqp.core.Binding;
import org.springframework.amqp.core.BindingBuilder;
import org.springframework.amqp.core.Queue;
import org.springframework.amqp.core.TopicExchange;
import org.springframework.amqp.rabbit.connection.CachingConnectionFactory;
import org.springframework.amqp.rabbit.connection.ConnectionFactory;
import org.springframework.amqp.rabbit.core.RabbitTemplate;
import org.springframework.amqp.support.converter.JacksonJsonMessageConverter;
import org.springframework.amqp.support.converter.MessageConverter;
import org.springframework.boot.autoconfigure.condition.ConditionalOnMissingBean;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;

/**
 * RabbitMQ infrastructure used by card-service.
 *
 * <p>The service publishes card lifecycle notifications and also consumes the
 * {@code card.create} event sent after account creation.
 */
@Configuration
public class RabbitConfig {

    /** Hostname RabbitMQ servera. */
    @Value("${spring.rabbitmq.host}")
    private String rabbitHost;

    /** Port RabbitMQ servera. */
    @Value("${spring.rabbitmq.port}")
    private int rabbitPort;

    /** Korisnicko ime za autentifikaciju na RabbitMQ serveru. */
    @Value("${spring.rabbitmq.username}")
    private String rabbitUsername;

    /** Lozinka za autentifikaciju na RabbitMQ serveru. */
    @Value("${spring.rabbitmq.password}")
    private String rabbitPassword;

    /**
     * Creates the shared topic exchange used for account and card events.
     *
     * @param exchangeName exchange name from configuration
     * @return durable topic exchange
     */
    @Bean
    public TopicExchange cardEventsExchange(
            @Value("${card.rabbit.auto.exchange}") String exchangeName
    ) {
        return new TopicExchange(exchangeName, true, false);
    }

    /**
     * Creates the durable queue consumed by the automatic-card listener.
     *
     * @param queueName queue name from configuration
     * @return durable queue
     */
    @Bean
    public Queue automaticCardCreationQueue(
            @Value("${card.rabbit.auto.queue}") String queueName
    ) {
        return new Queue(queueName, true);
    }

    /**
     * Binds the automatic-card queue to the shared exchange using the
     * {@code card.create} routing key.
     *
     * @param automaticCardCreationQueue listener queue bean
     * @param cardEventsExchange topic exchange bean
     * @param routingKey exact routing key for automatic card creation
     * @return exchange-to-queue binding
     */
    @Bean("card_automaticCardCreationBinding")
    public Binding automaticCardCreationBinding(
            Queue automaticCardCreationQueue,
            TopicExchange cardEventsExchange,
            @Value("${card.rabbit.auto.routing-key}") String routingKey
    ) {
        return BindingBuilder.bind(automaticCardCreationQueue)
                .to(cardEventsExchange)
                .with(routingKey);
    }

    /**
     * Kreira RabbitMQ connection factory na osnovu vrednosti iz konfiguracije.
     *
     * @return konekcioni factory za komunikaciju sa RabbitMQ serverom
     */
    @Bean
    @ConditionalOnMissingBean(ConnectionFactory.class)
    public ConnectionFactory connectionFactory() {
        CachingConnectionFactory connectionFactory = new CachingConnectionFactory(rabbitHost);
        connectionFactory.setPort(rabbitPort);
        connectionFactory.setUsername(rabbitUsername);
        connectionFactory.setPassword(rabbitPassword);
        return connectionFactory;
    }

    /**
     * Kreira RabbitTemplate i povezuje JSON konverter poruka.
     *
     * @param connectionFactory factory za otvaranje RabbitMQ konekcija
     * @param messageConverter konverter objekata u JSON poruke
     * @return konfigurisan RabbitTemplate
     */
    @Bean
    @ConditionalOnMissingBean(RabbitTemplate.class)
    public RabbitTemplate rabbitTemplate(ConnectionFactory connectionFactory, MessageConverter messageConverter) {
        RabbitTemplate template = new RabbitTemplate(connectionFactory);
        template.setMessageConverter(messageConverter);
        return template;
    }

    /**
     * Registers a JSON converter used both for publishing notifications and
     * consuming card creation events.
     *
     * @return Jackson-based message converter
     */
    @Bean
    @ConditionalOnMissingBean(MessageConverter.class)
    public MessageConverter messageConverter() {
        return new JacksonJsonMessageConverter();
    }
}
