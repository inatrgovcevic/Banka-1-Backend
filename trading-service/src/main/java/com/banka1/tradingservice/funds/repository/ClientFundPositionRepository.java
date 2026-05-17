package com.banka1.tradingservice.funds.repository;

import com.banka1.tradingservice.funds.domain.ClientFundPosition;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.Lock;
import org.springframework.data.jpa.repository.Query;
import org.springframework.data.repository.query.Param;
import org.springframework.stereotype.Repository;

import jakarta.persistence.LockModeType;
import java.util.List;
import java.util.Optional;

@Repository
public interface ClientFundPositionRepository extends JpaRepository<ClientFundPosition, Long> {

    Optional<ClientFundPosition> findByClientIdAndFundId(Long clientId, Long fundId);

    List<ClientFundPosition> findByClientId(Long clientId);

    List<ClientFundPosition> findByFundId(Long fundId);

    @Lock(LockModeType.PESSIMISTIC_WRITE)
    @Query("select p from ClientFundPosition p where p.clientId = :clientId and p.fundId = :fundId")
    Optional<ClientFundPosition> findByClientIdAndFundIdForUpdate(
            @Param("clientId") Long clientId, @Param("fundId") Long fundId);
}
