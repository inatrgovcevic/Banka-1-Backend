package com.banka1.banking_service.account_service.rabbitMQ;

import org.springframework.amqp.core.Binding;
import org.springframework.amqp.core.BindingBuilder;
import org.springframework.amqp.core.Queue;
import org.springframework.amqp.core.TopicExchange;
import org.springframework.amqp.rabbit.connection.CachingConnectionFactory;
import org.springframework.amqp.rabbit.connection.ConnectionFactory;
import org.springframework.amqp.rabbit.core.RabbitTemplate;
import org.springframework.amqp.support.converter.JacksonJsonMessageConverter;
import org.springframework.amqp.support.converter.MessageConverter;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;

/**
 * Spring konfiguracija RabbitMQ infrastrukture.
 * Definiše queue, direct exchange, binding i JSON konverter za serijalizaciju poruka.
 */
@Configuration
public class RabbitConfig {

    /** Naziv RabbitMQ queue-a na koji stizu email poruke. */
    @Value("${rabbitmq.queue}")
    private String queueName;

    /** Naziv direct exchange-a na koji se poruke objavljuju. */
    @Value("${rabbitmq.exchange}")
    private String exchangeName;

    /** Routing kljuc koji vezuje exchange za queue. */
    @Value("${rabbitmq.routing-key}")
    private String routingKey;

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
     * Kreira RabbitMQ connection factory na osnovu vrednosti iz konfiguracije.
     *
     * @return konekcioni factory za komunikaciju sa RabbitMQ serverom
     */
    @Bean
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
     * @param jacksonMessageConverter konverter objekata u JSON poruke
     * @return konfigurisan RabbitTemplate
     */
    @Bean
    public RabbitTemplate rabbitTemplate(ConnectionFactory connectionFactory, MessageConverter jacksonMessageConverter) {
        RabbitTemplate template = new RabbitTemplate(connectionFactory);
        template.setMessageConverter(jacksonMessageConverter);
        return template;
    }

    /**
     * Registruje Jackson konverter za serijalizaciju RabbitMQ poruka u JSON format.
     *
     * @return JSON message converter
     */
    @Bean
    public MessageConverter jacksonMessageConverter() {
        return new JacksonJsonMessageConverter();
    }

    /**
     * Kreira trajni RabbitMQ queue sa konfigurisanim nazivom.
     *
     * @return deklarisani durable queue
     */
    @Bean
    public Queue queue() {
        return new Queue(queueName, true);
    }

    /**
     * Kreira topic exchange za rutiranje notifikacija.
     *
     * @return deklarisani topic exchange
     */
    @Bean
    public TopicExchange topicExchange() {
        return new TopicExchange(exchangeName);
    }

    /**
     * Povezuje queue i exchange preko konfigurisanog routing kljuca.
     *
     * @param queue queue koji prima poruke
     * @param topicExchange exchange preko kog se poruke rutiraju
     * @return deklarisani binding
     */
    @Bean
    public Binding binding(Queue queue, TopicExchange topicExchange) {
        return BindingBuilder.bind(queue).to(topicExchange).with(routingKey);
    }
}
