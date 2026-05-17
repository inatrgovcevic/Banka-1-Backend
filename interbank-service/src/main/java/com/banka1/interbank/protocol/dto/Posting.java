package com.banka1.interbank.protocol.dto;

import jakarta.validation.constraints.NotNull;
import java.math.BigDecimal;

public record Posting(
    @NotNull TxAccount account,
    @NotNull BigDecimal amount,
    @NotNull Asset asset
) {}
