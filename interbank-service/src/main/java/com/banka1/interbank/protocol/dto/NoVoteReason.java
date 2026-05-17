package com.banka1.interbank.protocol.dto;

import com.fasterxml.jackson.annotation.JsonInclude;

@JsonInclude(JsonInclude.Include.NON_NULL)
public record NoVoteReason(Reason reason, Posting posting) {
    public enum Reason {
        UNBALANCED_TX,
        NO_SUCH_ACCOUNT,
        NO_SUCH_ASSET,
        UNACCEPTABLE_ASSET,
        INSUFFICIENT_ASSET,
        OPTION_AMOUNT_INCORRECT,
        OPTION_USED_OR_EXPIRED,
        OPTION_NEGOTIATION_NOT_FOUND
    }
    public static NoVoteReason of(Reason r) { return new NoVoteReason(r, null); }
    public static NoVoteReason of(Reason r, Posting p) { return new NoVoteReason(r, p); }
}
