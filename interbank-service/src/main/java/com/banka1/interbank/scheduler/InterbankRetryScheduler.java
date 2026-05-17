package com.banka1.interbank.scheduler;

import com.banka1.interbank.model.InterbankMessageEntity;
import com.banka1.interbank.model.enums.MessageStatus;
import com.banka1.interbank.repository.InterbankMessageRepository;
import com.banka1.interbank.service.InterbankClient;
import java.time.Duration;
import java.time.Instant;
import java.util.List;
import lombok.RequiredArgsConstructor;
import lombok.extern.slf4j.Slf4j;
import org.springframework.context.annotation.Profile;
import org.springframework.scheduling.annotation.Scheduled;
import org.springframework.stereotype.Component;
import org.springframework.transaction.annotation.Transactional;

/**
 * PR_32 Phase 8 Task 8.3: scheduled retry za OUTBOUND interbank poruke koje nisu uspesno
 * poslate u prvom pokusaju.
 *
 * <p>Algoritam (per Tim 2 §2.2, §6.5):
 * <ol>
 *   <li>Svake 2 minuta, query {@link InterbankMessageRepository#findOutboundPendingForRetry}
 *       za poruke koje su OUTBOUND, u PENDING_SEND/SENT statusu, sa retryCount &lt; 5 i
 *       lastAttemptAt &gt; 2 min staro (ili null).</li>
 *   <li>Za svaku takvu poruku, proveri backoff window ({@link #BACKOFFS_SECONDS} — 2/4/8/16/32s
 *       po retry-u). Preskoci ako jos nije isteklo.</li>
 *   <li>Pokusaj resend kroz {@link InterbankClient#resendByEntity}. Uspeh azurira status u
 *       SENT (kroz markSent u klijentu). Failure: increment retryCount, update lastAttemptAt,
 *       i ako je dostignut MAX_RETRIES (5), markiraj kao {@link MessageStatus#STUCK} —
 *       operator review.</li>
 * </ol>
 *
 * <p>NB: query uzima i status=SENT (sa transactionIdLocal IS NULL, indirektno kroz buducu
 * commit-confirmation logiku) ali u Phase 8 SENT poruke nisu predmet retry-a — to ostavljam
 * za buduce phase-ove. Trenutno samo PENDING_SEND poruke trigger-uju retry path.
 */
@Component
@Profile("!test")
@RequiredArgsConstructor
@Slf4j
public class InterbankRetryScheduler {

    /** Maksimalan broj retry pokusaja pre STUCK statusa (per Tim 2 §2.2). */
    static final int MAX_RETRIES = 5;

    /** Backoff intervali u sekundama, indeksirani retryCount-om (0-based). */
    static final long[] BACKOFFS_SECONDS = {2, 4, 8, 16, 32};

    private final InterbankMessageRepository repo;
    private final InterbankClient client;

    /**
     * Pokrece se na svakih 2 minuta (fixedRate = 120_000 ms). Cutoff za query je
     * {@code now - 2min} — poruke novije od 2 min se preskacu (svejedno bi backoff
     * window prevazisao samo na 5. retry-u sa 32s).
     */
    @Scheduled(fixedRate = 120_000)
    public void retryStaleMessages() {
        Instant cutoff = Instant.now().minus(Duration.ofMinutes(2));
        List<InterbankMessageEntity> pending = repo.findOutboundPendingForRetry(MAX_RETRIES, cutoff);
        if (pending.isEmpty()) {
            return;
        }
        log.info("Retry scheduler picked {} pending outbound messages", pending.size());
        for (InterbankMessageEntity msg : pending) {
            if (!shouldAttempt(msg)) {
                continue;
            }
            tryOnce(msg);
        }
    }

    /**
     * Proveri da li je istekao backoff window za ovu poruku.
     *
     * @return true ako je proteklo dovoljno vremena od poslednjeg pokusaja
     */
    boolean shouldAttempt(InterbankMessageEntity msg) {
        if (msg.getLastAttemptAt() == null) {
            return true;
        }
        long elapsedSec = (System.currentTimeMillis() - msg.getLastAttemptAt().toEpochMilli()) / 1000L;
        int backoffIdx = Math.min(msg.getRetryCount(), BACKOFFS_SECONDS.length - 1);
        return elapsedSec >= BACKOFFS_SECONDS[backoffIdx];
    }

    /**
     * Pokusaj jedan resend. Uspeh azurira status kroz {@code resendByEntity}'s markSent.
     * Failure inkrementuje retryCount i update lastAttemptAt; pri MAX_RETRIES markiraj STUCK.
     */
    @Transactional
    void tryOnce(InterbankMessageEntity msg) {
        try {
            client.resendByEntity(msg);
        } catch (Exception e) {
            msg.setRetryCount(msg.getRetryCount() + 1);
            msg.setLastAttemptAt(Instant.now());
            if (msg.getRetryCount() >= MAX_RETRIES) {
                msg.setStatus(MessageStatus.STUCK);
                log.error("Interbank message STUCK: id={} key={} retries={} cause={}",
                        msg.getId(), msg.getLocallyGeneratedKey(), msg.getRetryCount(),
                        e.getMessage());
            } else {
                log.warn("Interbank retry failed: id={} key={} attempt={} cause={}",
                        msg.getId(), msg.getLocallyGeneratedKey(), msg.getRetryCount(),
                        e.getMessage());
            }
            repo.save(msg);
        }
    }
}
