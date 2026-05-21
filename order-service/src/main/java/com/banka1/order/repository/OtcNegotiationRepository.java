package com.banka1.order.repository;

import com.banka1.order.entity.OtcNegotiation;
import com.banka1.order.entity.enums.OtcNegotiationStatus;
import jakarta.persistence.LockModeType;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.Lock;
import org.springframework.data.jpa.repository.Query;
import org.springframework.data.repository.query.Param;
import org.springframework.stereotype.Repository;

import java.time.LocalDate;
import java.util.List;
import java.util.Optional;

/**
 * Repository for OTC negotiations.
 */
@Repository
public interface OtcNegotiationRepository extends JpaRepository<OtcNegotiation, Long> {

    List<OtcNegotiation> findByBuyerIdOrSellerId(Long buyerId, Long sellerId);

    List<OtcNegotiation> findByStatusAndContractExpiryDate(OtcNegotiationStatus status, LocalDate contractExpiryDate);

    List<OtcNegotiation> findByStatusAndContractExpiryDateBefore(OtcNegotiationStatus status, LocalDate contractExpiryDate);

    @Lock(LockModeType.PESSIMISTIC_WRITE)
    @Query("select n from OtcNegotiation n where n.id = :id")
    Optional<OtcNegotiation> findByIdForUpdate(@Param("id") Long id);
}
