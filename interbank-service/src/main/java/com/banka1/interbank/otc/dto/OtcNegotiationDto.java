package com.banka1.interbank.otc.dto;

import com.banka1.interbank.protocol.dto.ForeignBankId;
import com.banka1.interbank.protocol.dto.MonetaryValue;
import com.banka1.interbank.protocol.dto.StockDescription;
import java.time.OffsetDateTime;

/**
 * PR_32 Phase 10 Task 10.1: OTC negotiation snapshot za Tim 2 §3.4
 * (GET /negotiations/{rn}/{id}). Razlikuje se od {@link OtcOfferDto} po
 * {@code isOngoing} flag-u koji govori da li je pregovor jos otvoren.
 */
public record OtcNegotiationDto(
        StockDescription stock,
        OffsetDateTime settlementDate,
        MonetaryValue pricePerUnit,
        MonetaryValue premium,
        ForeignBankId buyerId,
        ForeignBankId sellerId,
        int amount,
        ForeignBankId lastModifiedBy,
        boolean isOngoing
) {}
