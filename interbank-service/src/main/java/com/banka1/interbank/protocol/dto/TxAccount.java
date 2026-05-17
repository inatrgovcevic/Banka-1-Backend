package com.banka1.interbank.protocol.dto;

import com.fasterxml.jackson.annotation.JsonSubTypes;
import com.fasterxml.jackson.annotation.JsonTypeInfo;
import jakarta.validation.constraints.NotNull;
import jakarta.validation.constraints.Pattern;

@JsonTypeInfo(use = JsonTypeInfo.Id.NAME, property = "type")
@JsonSubTypes({
    @JsonSubTypes.Type(value = TxAccount.Person.class, name = "PERSON"),
    @JsonSubTypes.Type(value = TxAccount.Account.class, name = "ACCOUNT"),
    @JsonSubTypes.Type(value = TxAccount.Option.class, name = "OPTION")
})
public sealed interface TxAccount {
    record Person(@NotNull ForeignBankId id) implements TxAccount {}
    record Account(@NotNull @Pattern(regexp = "\\d{18}") String num) implements TxAccount {}
    record Option(@NotNull ForeignBankId id) implements TxAccount {}
}
