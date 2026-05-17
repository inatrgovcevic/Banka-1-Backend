package com.banka1.interbank.protocol.dto;

import jakarta.validation.constraints.NotBlank;
import jakarta.validation.constraints.Size;

public record StockDescription(@NotBlank @Size(max = 16) String ticker) {}
