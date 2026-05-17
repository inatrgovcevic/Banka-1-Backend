package com.banka1.interbank.otc.wrapper.dto;

import com.banka1.interbank.protocol.dto.CurrencyCode;
import jakarta.validation.constraints.Min;
import jakarta.validation.constraints.NotNull;
import java.math.BigDecimal;
import java.time.OffsetDateTime;

/**
 * PR_33 Phase A: FE-facing payload za PUT /api/interbank/otc/negotiations/{id}/counter.
 *
 * <p>Korisnik iz nase banke (poslednji nije bio mi) salje kontraponudu ka partner-u.
 * Kontroler load-uje local mirror, gradi {@link com.banka1.interbank.otc.dto.OtcOfferDto}
 * sa {@code lastModifiedBy = {myRouting, "C-{principalId}"}}, i PUT-uje ka partner-u
 * preko X-Api-Key.
 */
public record OutboundCounterOfferRequest(
        @Min(1) int amount,
        @NotNull CurrencyCode priceCurrency,
        @NotNull BigDecimal pricePerUnit,
        @NotNull CurrencyCode premiumCurrency,
        @NotNull BigDecimal premium,
        @NotNull OffsetDateTime settlementDate
) {}
