package com.banka1.interbank.repository;

import com.banka1.interbank.model.InterbankNegotiationEntity;
import java.util.List;
import org.springframework.data.jpa.repository.JpaRepository;

/**
 * PR_32 Phase 3: Spring Data repo za {@link InterbankNegotiationEntity}.
 *
 * <p>Glavni use-case-ovi:
 * <ul>
 *   <li>{@link #findByBuyerRoutingNumberAndBuyerId} — buyer-side "moje
 *       ponude koje sam poslao".</li>
 *   <li>{@link #findBySellerRoutingNumberAndSellerIdAndIsOngoing} —
 *       seller-side "ongoing ponude koje cekaju moj odgovor".</li>
 * </ul>
 */
public interface InterbankNegotiationRepository extends JpaRepository<InterbankNegotiationEntity, String> {

    List<InterbankNegotiationEntity> findByBuyerRoutingNumberAndBuyerId(
        int buyerRoutingNumber,
        String buyerId
    );

    List<InterbankNegotiationEntity> findBySellerRoutingNumberAndSellerIdAndIsOngoing(
        int sellerRoutingNumber,
        String sellerId,
        boolean isOngoing
    );
}
