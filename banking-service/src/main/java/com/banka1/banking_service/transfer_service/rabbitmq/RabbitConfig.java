package com.banka1.banking_service.transfer_service.rabbitmq;

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
 * Konfiguraciona klasa za RabbitMQ infrastrukturu.
 * Definiše konekcije, šablone za slanje, pretvaranje poruka u JSON format, kao i same Queue i Exchange strukture.
 */
@Configuration
public class RabbitConfig {

    @Value("${rabbitmq.queue}")
    private String queueName;

    @Value("${rabbitmq.exchange}")
    private String exchangeName;

    @Value("${rabbitmq.routing-key}")
    private String routingKey;

    @Value("${spring.rabbitmq.host}")
    private String rabbitHost;

    @Value("${spring.rabbitmq.port}")
    private int rabbitPort;

    @Value("${spring.rabbitmq.username}")
    private String rabbitUsername;

    @Value("${spring.rabbitmq.password}")
    private String rabbitPassword;

    /** Kreira fabriku konekcija sa parametrima učitanim iz okruženja. */
    @Bean
    @ConditionalOnMissingBean(ConnectionFactory.class)
    public ConnectionFactory connectionFactory() {
        CachingConnectionFactory connectionFactory = new CachingConnectionFactory(rabbitHost);
        connectionFactory.setPort(rabbitPort);
        connectionFactory.setUsername(rabbitUsername);
        connectionFactory.setPassword(rabbitPassword);
        return connectionFactory;
    }

    /** Konfiguriše RabbitTemplate sa podrškom za JSON konverziju poruka. */
    @Bean
    @ConditionalOnMissingBean(RabbitTemplate.class)
    public RabbitTemplate rabbitTemplate(ConnectionFactory connectionFactory, MessageConverter jacksonMessageConverter) {
        RabbitTemplate template = new RabbitTemplate(connectionFactory);
        template.setMessageConverter(jacksonMessageConverter);
        return template;
    }

    /** Postavlja Jackson konvertor kako bi se DTO objekti automatski serijalizovali u JSON. */
    @Bean
    @ConditionalOnMissingBean(MessageConverter.class)
    public MessageConverter jacksonMessageConverter() {
        return new JacksonJsonMessageConverter();
    }

    /** Definiše perzistentni red poruka (Queue) za notifikacije. */
    @Bean
    @ConditionalOnMissingBean(name = "queue")
    public Queue queue() {
        return new Queue(queueName, true);
    }

    /** Definiše Topic Exchange za fleksibilno rutiranje poruka. */
    @Bean
    @ConditionalOnMissingBean(name = "topicExchange")
    public TopicExchange topicExchange() {
        return new TopicExchange(exchangeName);
    }

    /** Povezuje Queue sa Exchange-om koristeći definisani routing key. */
    @Bean
    @ConditionalOnMissingBean(name = "binding")
    public Binding binding(Queue queue, TopicExchange topicExchange) {
        // Zavezuje queue za exchange pomoću specifičnog ključa (npr. transfer.#)
        return BindingBuilder.bind(queue).to(topicExchange).with(routingKey);
    }
}
