package com.banka1.interbank.scheduler;

import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.Mockito.doNothing;
import static org.mockito.Mockito.doThrow;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

import com.banka1.interbank.model.InterbankMessageEntity;
import com.banka1.interbank.model.enums.Direction;
import com.banka1.interbank.model.enums.MessageStatus;
import com.banka1.interbank.protocol.dto.MessageType;
import com.banka1.interbank.repository.InterbankMessageRepository;
import com.banka1.interbank.service.InterbankClient;
import java.time.Instant;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.InjectMocks;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;
import org.springframework.web.client.RestClientException;

/**
 * PR_32 Phase 8 unit testovi za {@link InterbankRetryScheduler}.
 */
@ExtendWith(MockitoExtension.class)
class InterbankRetrySchedulerTest {

    @Mock
    private InterbankMessageRepository repo;

    @Mock
    private InterbankClient client;

    @InjectMocks
    private InterbankRetryScheduler scheduler;

    private InterbankMessageEntity baseEntity;

    @BeforeEach
    void setUp() {
        baseEntity = InterbankMessageEntity.builder()
                .id(1L)
                .direction(Direction.OUTBOUND)
                .senderRoutingNumber(222)
                .locallyGeneratedKey("abcd1234")
                .messageType(MessageType.NEW_TX)
                .status(MessageStatus.PENDING_SEND)
                .requestBody("{}")
                .retryCount(0)
                .lastAttemptAt(null)
                .build();
    }

    @Test
    void shouldAttempt_returnsTrueWhenLastAttemptIsNull() {
        baseEntity.setLastAttemptAt(null);
        assertThat(scheduler.shouldAttempt(baseEntity)).isTrue();
    }

    @Test
    void shouldAttempt_returnsFalseWhenWithinBackoffWindow() {
        // retryCount=0 → backoff=2s. lastAttemptAt = now − 1s. Backoff nije istekao.
        baseEntity.setRetryCount(0);
        baseEntity.setLastAttemptAt(Instant.now().minusMillis(1_000));
        assertThat(scheduler.shouldAttempt(baseEntity)).isFalse();
    }

    @Test
    void shouldAttempt_returnsTrueWhenPastBackoffWindow() {
        // retryCount=0 → backoff=2s. lastAttemptAt = now − 5s. Backoff istekao.
        baseEntity.setRetryCount(0);
        baseEntity.setLastAttemptAt(Instant.now().minusSeconds(5));
        assertThat(scheduler.shouldAttempt(baseEntity)).isTrue();
    }

    @Test
    void shouldAttempt_usesLargestBackoffForRetryCountAboveTable() {
        // retryCount=10 → idx min(10, 4)=4 → backoff=32s. lastAttemptAt = now − 20s. Still in window.
        baseEntity.setRetryCount(10);
        baseEntity.setLastAttemptAt(Instant.now().minusSeconds(20));
        assertThat(scheduler.shouldAttempt(baseEntity)).isFalse();

        baseEntity.setLastAttemptAt(Instant.now().minusSeconds(40));
        assertThat(scheduler.shouldAttempt(baseEntity)).isTrue();
    }

    @Test
    void tryOnce_success_delegatesToClientAndDoesNotIncrementRetry() {
        doNothing().when(client).resendByEntity(any(InterbankMessageEntity.class));

        scheduler.tryOnce(baseEntity);

        verify(client).resendByEntity(baseEntity);
        // Scheduler ne dira retryCount kad uspe — markSent u klijentu radi update statusa.
        assertThat(baseEntity.getRetryCount()).isZero();
    }

    @Test
    void tryOnce_failure_incrementsRetryCountAndSaves() {
        doThrow(new RestClientException("partner down"))
                .when(client).resendByEntity(any(InterbankMessageEntity.class));
        when(repo.save(any(InterbankMessageEntity.class))).thenAnswer(inv -> inv.getArgument(0));

        baseEntity.setRetryCount(1);
        scheduler.tryOnce(baseEntity);

        assertThat(baseEntity.getRetryCount()).isEqualTo(2);
        assertThat(baseEntity.getStatus()).isEqualTo(MessageStatus.PENDING_SEND);
        assertThat(baseEntity.getLastAttemptAt()).isNotNull();
        verify(repo).save(baseEntity);
    }

    @Test
    void tryOnce_fifthFailure_marksStuck() {
        doThrow(new RestClientException("partner down"))
                .when(client).resendByEntity(any(InterbankMessageEntity.class));
        when(repo.save(any(InterbankMessageEntity.class))).thenAnswer(inv -> inv.getArgument(0));

        // retryCount=4 prelazi u 5 = MAX_RETRIES.
        baseEntity.setRetryCount(4);
        scheduler.tryOnce(baseEntity);

        assertThat(baseEntity.getRetryCount()).isEqualTo(5);
        assertThat(baseEntity.getStatus()).isEqualTo(MessageStatus.STUCK);
        verify(repo).save(baseEntity);
    }
}
