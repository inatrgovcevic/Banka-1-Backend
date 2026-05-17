package com.banka1.order.repository;

import com.banka1.order.entity.Transaction;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.Query;
import org.springframework.data.repository.query.Param;
import org.springframework.stereotype.Repository;

import java.math.BigDecimal;
import java.time.LocalDateTime;
import java.util.Collection;
import java.util.List;

/**
 * Repository for {@link Transaction} entities.
 */
@Repository
public interface TransactionRepository extends JpaRepository<Transaction, Long> {

    /**
     * Returns all executed transaction portions for a given order.
     *
     * @param orderId the parent order's identifier
     * @return list of transactions in chronological insertion order
     */
    List<Transaction> findByOrderId(Long orderId);

    /**
     * Returns transactions executed between two timestamps (inclusive start, exclusive end).
     * Useful for monthly tax calculation.
     */
    List<Transaction> findByTimestampBetween(LocalDateTime start, LocalDateTime end);

    List<Transaction> findByOrderIdInAndTimestampBetween(Collection<Long> orderIds, LocalDateTime start, LocalDateTime end);

    List<Transaction> findByOrderIdInAndTimestampBefore(Collection<Long> orderIds, LocalDateTime end);

    /**
     * Suma komisija po aktuaru (Order.userId koji je placed-or). PR_14 C14.9.
     * Vraca [userId, totalCommission, transactionCount] za sve agente koji su
     * imali bar jednu izvrsenu transakciju u zadatom intervalu.
     */
    // PR_021: Postgres ne moze da odredi tip ":from is null" pattern (PSQLException),
    // pa caller (ActuaryServiceImpl) substituira null sentinel-vrednostima.
    @Query("""
        select o.userId as userId,
               coalesce(sum(t.commission), 0) as totalCommission,
               count(t) as transactionCount
          from Transaction t
          join Order o on o.id = t.orderId
         where t.timestamp >= :from and t.timestamp < :to
         group by o.userId
         order by totalCommission desc
        """)
    List<ActuaryProfitRow> sumCommissionByActuary(
            @Param("from") LocalDateTime from,
            @Param("to") LocalDateTime to);

    interface ActuaryProfitRow {
        Long getUserId();
        BigDecimal getTotalCommission();
        Long getTransactionCount();
    }
}
