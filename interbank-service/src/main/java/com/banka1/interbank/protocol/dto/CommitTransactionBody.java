package com.banka1.interbank.protocol.dto;

import jakarta.validation.constraints.NotNull;

public record CommitTransactionBody(@NotNull ForeignBankId transactionId) {}
