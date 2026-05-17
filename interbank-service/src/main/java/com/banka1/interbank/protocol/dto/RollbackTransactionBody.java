package com.banka1.interbank.protocol.dto;

import jakarta.validation.constraints.NotNull;

public record RollbackTransactionBody(@NotNull ForeignBankId transactionId) {}
