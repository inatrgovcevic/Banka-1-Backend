package com.banka1.interbank.protocol.dto;

import jakarta.validation.constraints.NotBlank;
import jakarta.validation.constraints.Size;

public record IdempotenceKey(
    int routingNumber,
    @NotBlank @Size(max = 64) String locallyGeneratedKey
) {}
