package com.banka1.interbank.protocol.dto;

import jakarta.validation.constraints.Min;
import jakarta.validation.constraints.NotNull;
import java.time.OffsetDateTime;

public record OptionDescription(
    @NotNull ForeignBankId negotiationId,
    @NotNull StockDescription stock,
    @NotNull MonetaryValue pricePerUnit,
    @NotNull OffsetDateTime settlementDate,
    @Min(1) int amount
) {}
