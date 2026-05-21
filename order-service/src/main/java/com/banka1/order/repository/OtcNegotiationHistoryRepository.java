package com.banka1.order.repository;

import com.banka1.order.entity.OtcNegotiationHistory;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.stereotype.Repository;

import java.util.List;

/**
 * Repository for append-only OTC negotiation history.
 */
@Repository
public interface OtcNegotiationHistoryRepository extends JpaRepository<OtcNegotiationHistory, Long> {

    List<OtcNegotiationHistory> findByNegotiationIdOrderByChangedAtAscIdAsc(Long negotiationId);
}
