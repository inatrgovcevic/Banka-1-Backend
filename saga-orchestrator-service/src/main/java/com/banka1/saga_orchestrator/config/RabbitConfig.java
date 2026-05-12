package com.banka1.saga_orchestrator.config;

import org.springframework.amqp.core.Binding;
import org.springframework.amqp.core.BindingBuilder;
import org.springframework.amqp.core.Queue;
import org.springframework.amqp.core.QueueBuilder;
import org.springframework.amqp.core.TopicExchange;
import org.springframework.amqp.rabbit.connection.ConnectionFactory;
import org.springframework.amqp.rabbit.core.RabbitTemplate;
import org.springframework.amqp.support.converter.Jackson2JsonMessageConverter;
import org.springframework.amqp.support.converter.MessageConverter;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;

/**
 * SAGA RabbitMQ topologija (Issue #215).
 *
 * <p>Konvencije:
 * <ul>
 *   <li>Topic exchange: {@code saga.exchange}.</li>
 *   <li>Komandni queue-ovi (orchestrator -&gt; servis):
 *       {@code saga.cmd.banking}, {@code saga.cmd.order},
 *       {@code saga.cmd.otc}, {@code saga.cmd.fund}.</li>
 *   <li>Event queue (servis -&gt; orchestrator): {@code saga.events}.</li>
 *   <li>DLQ: {@code saga.dlq} - poruke kojima istekne {@code retryCount} idu ovde
 *       (vidi {@code x-dead-letter-exchange} parametar svakog queue-a).</li>
 *   <li>Routing key: {@code saga.<sagaType>.<step>.<command|event>}.</li>
 *   <li>{@code correlationId == sagaInstanceId}.</li>
 * </ul>
 */
@Configuration
public class RabbitConfig {

    public static final String DLX_HEADER = "x-dead-letter-exchange";
    public static final String DLR_HEADER = "x-dead-letter-routing-key";

    @Value("${saga.rabbit.exchange:saga.exchange}")
    private String exchangeName;

    @Value("${saga.rabbit.dlq:saga.dlq}")
    private String dlqName;

    @Value("${saga.rabbit.events-queue:saga.events}")
    private String eventsQueue;

    @Value("${saga.rabbit.cmd-banking-queue:saga.cmd.banking}")
    private String cmdBankingQueue;

    @Value("${saga.rabbit.cmd-order-queue:saga.cmd.order}")
    private String cmdOrderQueue;

    @Value("${saga.rabbit.cmd-otc-queue:saga.cmd.otc}")
    private String cmdOtcQueue;

    @Value("${saga.rabbit.cmd-fund-queue:saga.cmd.fund}")
    private String cmdFundQueue;

    @Bean
    public TopicExchange sagaExchange() {
        return new TopicExchange(exchangeName, true, false);
    }

    @Bean
    public Queue sagaDlq() {
        return QueueBuilder.durable(dlqName).build();
    }

    @Bean
    public Queue sagaEventsQueue() {
        return durableWithDlq(eventsQueue);
    }

    @Bean
    public Queue cmdBankingQueue() {
        return durableWithDlq(cmdBankingQueue);
    }

    @Bean
    public Queue cmdOrderQueue() {
        return durableWithDlq(cmdOrderQueue);
    }

    @Bean
    public Queue cmdOtcQueue() {
        return durableWithDlq(cmdOtcQueue);
    }

    @Bean
    public Queue cmdFundQueue() {
        return durableWithDlq(cmdFundQueue);
    }

    private Queue durableWithDlq(String name) {
        return QueueBuilder.durable(name)
                .withArgument(DLX_HEADER, "")
                .withArgument(DLR_HEADER, dlqName)
                .build();
    }

    /**
     * Routing key konvencija {@code saga.<sagaType>.<step>.<command|event>}:
     * <ul>
     *   <li>{@code saga.OTC_EXERCISE.RESERVE_FUNDS.command}</li>
     *   <li>{@code saga.OTC_EXERCISE.RESERVE_FUNDS.success}</li>
     *   <li>{@code saga.FUND_LIQUIDATION_FOR_REDEMPTION.LIQUIDATE.failure}</li>
     * </ul>
     * Komandne queue dobijaju {@code saga.<sagaType>.*.command}, events queue
     * dobija {@code saga.*.*.success} i {@code saga.*.*.failure}. Konkretni
     * step routing key prefiks ide kroz {@link Binding} po servisu.
     */
    @Bean
    public Binding bindBankingCmd(Queue cmdBankingQueue, TopicExchange sagaExchange) {
        return BindingBuilder.bind(cmdBankingQueue).to(sagaExchange).with("saga.*.*.banking.command");
    }

    @Bean
    public Binding bindOrderCmd(Queue cmdOrderQueue, TopicExchange sagaExchange) {
        return BindingBuilder.bind(cmdOrderQueue).to(sagaExchange).with("saga.*.*.order.command");
    }

    @Bean
    public Binding bindOtcCmd(Queue cmdOtcQueue, TopicExchange sagaExchange) {
        return BindingBuilder.bind(cmdOtcQueue).to(sagaExchange).with("saga.*.*.otc.command");
    }

    @Bean
    public Binding bindFundCmd(Queue cmdFundQueue, TopicExchange sagaExchange) {
        return BindingBuilder.bind(cmdFundQueue).to(sagaExchange).with("saga.*.*.fund.command");
    }

    @Bean
    public Binding bindEventsSuccess(Queue sagaEventsQueue, TopicExchange sagaExchange) {
        return BindingBuilder.bind(sagaEventsQueue).to(sagaExchange).with("saga.*.*.*.success");
    }

    @Bean
    public Binding bindEventsFailure(Queue sagaEventsQueue, TopicExchange sagaExchange) {
        return BindingBuilder.bind(sagaEventsQueue).to(sagaExchange).with("saga.*.*.*.failure");
    }

    /**
     * PR_11 C11.2: TRIGGER bindings — kada drugi servisi (trading-service OtcService,
     * InvestmentFundService) zelu da pokreni novu saga-u, publish-uju event na
     * routing key {@code saga.<SAGA_TYPE>.START.command}; orchestrator konzumira
     * iz {@code saga.events} queue-a kroz {@link com.banka1.saga_orchestrator.listener.SagaEventListener}.
     */
    @Bean
    public Binding bindEventsTrigger(Queue sagaEventsQueue, TopicExchange sagaExchange) {
        return BindingBuilder.bind(sagaEventsQueue).to(sagaExchange).with("saga.*.START.command");
    }

    @Bean
    public MessageConverter jacksonMessageConverter() {
        return new Jackson2JsonMessageConverter();
    }

    @Bean
    public RabbitTemplate rabbitTemplate(ConnectionFactory connectionFactory, MessageConverter converter) {
        RabbitTemplate template = new RabbitTemplate(connectionFactory);
        template.setMessageConverter(converter);
        template.setExchange(exchangeName);
        return template;
    }
}
