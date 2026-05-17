package com.banka1.userservice.config;

import org.springframework.amqp.rabbit.core.RabbitTemplate;
import org.springframework.amqp.rabbit.connection.ConnectionFactory;
import org.springframework.amqp.support.converter.JacksonJsonMessageConverter;
import org.springframework.amqp.support.converter.MessageConverter;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import org.springframework.context.annotation.Primary;

@Configuration
public class RabbitConfig {

    @Bean
    public MessageConverter userServiceJacksonMessageConverter() {
        return new JacksonJsonMessageConverter();
    }

    @Bean
    @Primary
    public RabbitTemplate userServiceRabbitTemplate(ConnectionFactory connectionFactory,
                                                    MessageConverter userServiceJacksonMessageConverter) {
        RabbitTemplate template = new RabbitTemplate(connectionFactory);
        template.setMessageConverter(userServiceJacksonMessageConverter);
        return template;
    }
}
