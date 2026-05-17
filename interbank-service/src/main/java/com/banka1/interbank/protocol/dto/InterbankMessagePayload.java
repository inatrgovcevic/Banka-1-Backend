package com.banka1.interbank.protocol.dto;

import com.fasterxml.jackson.databind.JsonNode;
import jakarta.validation.Valid;
import jakarta.validation.constraints.NotNull;

public record InterbankMessagePayload(
    @NotNull @Valid IdempotenceKey idempotenceKey,
    @NotNull MessageType messageType,
    @NotNull JsonNode message
) {}
