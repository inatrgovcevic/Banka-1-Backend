package com.banka1.interbank.protocol.dto;

import jakarta.validation.Valid;
import jakarta.validation.constraints.NotEmpty;
import jakarta.validation.constraints.NotNull;
import java.util.List;

public record InterbankTransactionPayload(
    @NotEmpty @Valid List<Posting> postings,
    @NotNull ForeignBankId transactionId,
    String message,
    String callNumber,
    String paymentCode,
    String paymentPurpose
) {}
