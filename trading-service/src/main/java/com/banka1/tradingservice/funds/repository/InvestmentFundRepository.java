package com.banka1.tradingservice.funds.repository;

import com.banka1.tradingservice.funds.domain.InvestmentFund;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.Lock;
import org.springframework.data.jpa.repository.Query;
import org.springframework.data.repository.query.Param;
import org.springframework.stereotype.Repository;

import jakarta.persistence.LockModeType;
import java.util.List;
import java.util.Optional;

@Repository
public interface InvestmentFundRepository extends JpaRepository<InvestmentFund, Long> {

    List<InvestmentFund> findByDeletedFalseOrderByNazivAsc();

    /** Pessimistic-write lock pri redeem-u radi sprecavanja race-a sa drugim isplatama. */
    @Lock(LockModeType.PESSIMISTIC_WRITE)
    @Query("select f from InvestmentFund f where f.id = :id and f.deleted = false")
    Optional<InvestmentFund> findByIdForUpdate(@Param("id") Long id);

    List<InvestmentFund> findByManagerIdAndDeletedFalse(Long managerId);
}
