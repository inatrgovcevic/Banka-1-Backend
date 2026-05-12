package com.banka1.saga_orchestrator.config;

import org.junit.jupiter.api.Test;
import org.springframework.amqp.core.Binding;
import org.springframework.amqp.core.Queue;
import org.springframework.amqp.core.TopicExchange;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.test.context.SpringBootTest;
import org.springframework.context.ApplicationContext;
import org.springframework.test.context.TestPropertySource;

import java.util.Map;

import static org.assertj.core.api.Assertions.assertThat;

/**
 * Verifikuje da {@link RabbitConfig} deklaracije queue-ova i binding-a po DoD #215.
 */
@SpringBootTest(classes = RabbitConfig.class)
@TestPropertySource(properties = {
        "spring.rabbitmq.host=localhost",
        "spring.rabbitmq.port=5672"
})
class RabbitConfigTest {

    @Autowired
    private ApplicationContext ctx;

    @Test
    void declaresExchangeAndAllQueues() {
        TopicExchange exchange = ctx.getBean("sagaExchange", TopicExchange.class);
        assertThat(exchange.getName()).isEqualTo("saga.exchange");
        assertThat(exchange.isDurable()).isTrue();

        Map<String, Queue> queues = ctx.getBeansOfType(Queue.class);
        assertThat(queues.values().stream().map(Queue::getName))
                .contains("saga.dlq", "saga.events", "saga.cmd.banking",
                          "saga.cmd.order", "saga.cmd.otc", "saga.cmd.fund");
    }

    @Test
    void allCommandQueuesHaveDeadLetterToDlq() {
        Map<String, Queue> queues = ctx.getBeansOfType(Queue.class);
        queues.values().stream()
                .filter(q -> q.getName().startsWith("saga.cmd.") || q.getName().equals("saga.events"))
                .forEach(q -> {
                    Map<String, Object> args = q.getArguments();
                    assertThat(args).containsEntry(RabbitConfig.DLR_HEADER, "saga.dlq");
                });
    }

    @Test
    void exchangeBindingsExistForEachQueue() {
        Map<String, Binding> bindings = ctx.getBeansOfType(Binding.class);
        assertThat(bindings.values().stream().map(Binding::getRoutingKey))
                .contains(
                        "saga.*.*.banking.command",
                        "saga.*.*.order.command",
                        "saga.*.*.otc.command",
                        "saga.*.*.fund.command",
                        "saga.*.*.*.success",
                        "saga.*.*.*.failure"
                );
    }
}
