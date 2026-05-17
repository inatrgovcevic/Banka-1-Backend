package com.banka1.interbank.otc.wrapper.dto;

import com.banka1.interbank.protocol.dto.CurrencyCode;
import com.banka1.interbank.protocol.dto.ForeignBankId;
import jakarta.validation.Valid;
import jakarta.validation.constraints.Min;
import jakarta.validation.constraints.NotBlank;
import jakarta.validation.constraints.NotNull;
import jakarta.validation.constraints.Size;
import java.math.BigDecimal;
import java.time.OffsetDateTime;

/**
 * PR_33 Phase A: FE-facing payload za POST /api/interbank/otc/negotiations.
 *
 * <p>Klijent (buyer) iz <em>nase</em> banke salje ponudu ka seller-u u drugoj
 * banci. Pre nego sto stigne do partner-a, kontroler ovo konvertuje u
 * {@link com.banka1.interbank.otc.dto.OtcOfferDto} sa nasim X-Api-Key
 * outbound tokenom.
 *
 * <p>{@code sellerForeignBankId.routingNumber} MORA biti razlicit od
 * {@code interbank.my-routing-number} (mi smo buyer-bank, ne seller).
 * Validira se u service layer-u.
 *
 * <p>{@code buyerLocalUserId} je opcioni — ako nedostaje, kontroler ga vadi
 * iz JWT principal-a ({@code jwt.getClaim("id")}). Ovo daje supervisorima
 * mogucnost da iniciraju pregovor "u ime klijenta" (PR_33 Phase B fol-up).
 */
public record OutboundCreateNegotiationRequest(
        @NotBlank @Size(max = 16) String stockTicker,
        @NotNull OffsetDateTime settlementDate,
        @NotNull CurrencyCode priceCurrency,
        @NotNull BigDecimal pricePerUnit,
        @NotNull CurrencyCode premiumCurrency,
        @NotNull BigDecimal premium,
        @NotNull @Valid ForeignBankId sellerForeignBankId,
        @Min(1) int amount,
        Long buyerLocalUserId
) {}
