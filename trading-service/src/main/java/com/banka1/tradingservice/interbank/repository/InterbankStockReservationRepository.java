package com.banka1.tradingservice.interbank.repository;

import com.banka1.tradingservice.interbank.model.InterbankStockReservation;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.stereotype.Repository;

import java.util.Optional;
import java.util.UUID;

@Repository
public interface InterbankStockReservationRepository
        extends JpaRepository<InterbankStockReservation, Long> {

    Optional<InterbankStockReservation> findByReservationId(UUID reservationId);
}
