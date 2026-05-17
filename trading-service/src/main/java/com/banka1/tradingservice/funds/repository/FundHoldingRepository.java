package com.banka1.tradingservice.funds.repository;

import com.banka1.tradingservice.funds.domain.FundHolding;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.stereotype.Repository;

import java.util.List;
import java.util.Optional;

@Repository
public interface FundHoldingRepository extends JpaRepository<FundHolding, Long> {

    List<FundHolding> findByFundIdAndDeletedFalse(Long fundId);

    Optional<FundHolding> findByFundIdAndStockTickerAndDeletedFalse(Long fundId, String stockTicker);
}
