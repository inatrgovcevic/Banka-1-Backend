package com.banka1.interbank.protocol.dto;

import java.math.BigDecimal;
import jakarta.validation.constraints.NotNull;

public record MonetaryValue(@NotNull CurrencyCode currency, @NotNull BigDecimal amount) {}
