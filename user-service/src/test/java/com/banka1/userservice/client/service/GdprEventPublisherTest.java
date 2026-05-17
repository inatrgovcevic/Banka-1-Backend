package com.banka1.userservice.client.service;

import com.banka1.clientService.service.GdprEventPublisher;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.InjectMocks;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;
import org.springframework.amqp.rabbit.core.RabbitTemplate;
import org.springframework.test.util.ReflectionTestUtils;

import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.eq;
import static org.mockito.Mockito.verify;

@ExtendWith(MockitoExtension.class)
class GdprEventPublisherTest {

    @Mock private RabbitTemplate rabbitTemplate;

    @InjectMocks private GdprEventPublisher publisher;

    @Test
    void publishClientSoftDeleted_publish_event_kada_nema_aktivne_transakcije() {
        ReflectionTestUtils.setField(publisher, "exchange", "gdpr.events");

        publisher.publishClientSoftDeleted(100L, "user-requested");

        verify(rabbitTemplate).convertAndSend(eq("gdpr.events"),
                eq("gdpr.client.soft-deleted"), any(Object.class));
    }
}
