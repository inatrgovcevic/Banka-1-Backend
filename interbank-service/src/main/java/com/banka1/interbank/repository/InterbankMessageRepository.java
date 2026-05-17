package com.banka1.interbank.repository;

import com.banka1.interbank.model.InterbankMessageEntity;
import com.banka1.interbank.model.enums.Direction;
import java.time.Instant;
import java.util.List;
import java.util.Optional;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.Query;
import org.springframework.data.repository.query.Param;

/**
 * PR_32 Phase 3: Spring Data repo za {@link InterbankMessageEntity}.
 *
 * <p>Glavna upotreba:
 * <ul>
 *   <li>{@link #findByDirectionAndSenderRoutingNumberAndLocallyGeneratedKey} —
 *       lookup za idempotency cache (vraca cached response na retry).</li>
 *   <li>{@link #findOutboundPendingForRetry} — query za retry scheduler-a;
 *       hvata OUTBOUND poruke koje su PENDING_SEND ili SENT-bez-confirma i
 *       jos imaju retries pre nego sto skliznu u STUCK.</li>
 * </ul>
 */
public interface InterbankMessageRepository extends JpaRepository<InterbankMessageEntity, Long> {

    Optional<InterbankMessageEntity> findByDirectionAndSenderRoutingNumberAndLocallyGeneratedKey(
        Direction direction,
        int senderRoutingNumber,
        String locallyGeneratedKey
    );

    @Query("SELECT m FROM InterbankMessageEntity m "
        + "WHERE m.direction = com.banka1.interbank.model.enums.Direction.OUTBOUND "
        + "AND m.status IN (com.banka1.interbank.model.enums.MessageStatus.PENDING_SEND, "
        + "                 com.banka1.interbank.model.enums.MessageStatus.SENT) "
        + "AND m.retryCount < :maxRetries "
        + "AND (m.lastAttemptAt IS NULL OR m.lastAttemptAt < :cutoff) "
        + "ORDER BY m.lastAttemptAt ASC NULLS FIRST")
    List<InterbankMessageEntity> findOutboundPendingForRetry(
        @Param("maxRetries") int maxRetries,
        @Param("cutoff") Instant cutoff
    );
}
