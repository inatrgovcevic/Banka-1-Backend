package com.banka1.interbank.otc.dto;

import com.banka1.interbank.protocol.dto.ForeignBankId;
import com.banka1.interbank.protocol.dto.MonetaryValue;
import com.banka1.interbank.protocol.dto.StockDescription;
import jakarta.validation.Valid;
import jakarta.validation.constraints.Min;
import jakarta.validation.constraints.NotNull;
import java.time.OffsetDateTime;

/**
 * PR_32 Phase 10 Task 10.1: OTC offer payload za Tim 2 §3.2 (POST /negotiations)
 * i §3.3 (PUT /negotiations/{rn}/{id} counter-offer).
 *
 * <p>{@code lastModifiedBy} je ForeignBankId pozivaoca (mora se poklapati sa
 * sender-om iz X-Api-Key autentifikacije — vidi
 * {@link com.banka1.interbank.service.OtcNegotiationService#updateCounter}).
 */
public record OtcOfferDto(
        @NotNull @Valid StockDescription stock,
        @NotNull OffsetDateTime settlementDate,
        @NotNull @Valid MonetaryValue pricePerUnit,
        @NotNull @Valid MonetaryValue premium,
        @NotNull @Valid ForeignBankId buyerId,
        @NotNull @Valid ForeignBankId sellerId,
        @Min(1) int amount,
        @NotNull @Valid ForeignBankId lastModifiedBy
) {}
