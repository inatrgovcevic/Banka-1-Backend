package com.banka1.interbank.protocol.dto;

import jakarta.validation.constraints.NotBlank;
import jakarta.validation.constraints.Size;

public record ForeignBankId(
    int routingNumber,
    @NotBlank @Size(max = 64) String id
) {}
