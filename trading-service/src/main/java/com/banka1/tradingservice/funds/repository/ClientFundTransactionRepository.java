package com.banka1.tradingservice.funds.repository;

import com.banka1.tradingservice.funds.domain.ClientFundTransaction;
import com.banka1.tradingservice.funds.domain.ClientFundTransactionStatus;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.stereotype.Repository;

import java.util.List;

@Repository
public interface ClientFundTransactionRepository extends JpaRepository<ClientFundTransaction, Long> {

    List<ClientFundTransaction> findByClientIdOrderByOccurredAtDesc(Long clientId);
    List<ClientFundTransaction> findByFundIdOrderByOccurredAtDesc(Long fundId);
    List<ClientFundTransaction> findByStatus(ClientFundTransactionStatus status);
}
