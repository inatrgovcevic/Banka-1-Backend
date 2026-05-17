package com.banka1.interbank.otc.dto;

import com.banka1.interbank.protocol.dto.ForeignBankId;

/**
 * PR_32 Phase 5 (stub za Phase 10): jedna stavka prodavca u public-stock
 * listi (per Tim 2 §11 spec).
 *
 * @param seller foreign bank ID prodavca + njegov lokalni user ID
 * @param amount kolicina ponudjenih akcija (mora biti pozitivna)
 */
public record PublicStockSellerDto(ForeignBankId seller, int amount) {}
