package com.banka1.interbank.protocol.dto;

import com.fasterxml.jackson.annotation.JsonSubTypes;
import com.fasterxml.jackson.annotation.JsonTypeInfo;
import jakarta.validation.constraints.NotNull;

@JsonTypeInfo(use = JsonTypeInfo.Id.NAME, property = "type")
@JsonSubTypes({
    @JsonSubTypes.Type(value = Asset.Monas.class, name = "MONAS"),
    @JsonSubTypes.Type(value = Asset.Stock.class, name = "STOCK"),
    @JsonSubTypes.Type(value = Asset.Option.class, name = "OPTION")
})
public sealed interface Asset {
    record Monas(@NotNull MonetaryAsset asset) implements Asset {}
    record Stock(@NotNull StockDescription asset) implements Asset {}
    record Option(@NotNull OptionDescription asset) implements Asset {}
}
