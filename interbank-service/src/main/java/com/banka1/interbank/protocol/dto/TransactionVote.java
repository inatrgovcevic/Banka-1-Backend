package com.banka1.interbank.protocol.dto;

import com.fasterxml.jackson.annotation.JsonIgnore;
import com.fasterxml.jackson.annotation.JsonInclude;
import java.util.List;

@JsonInclude(JsonInclude.Include.NON_NULL)
public record TransactionVote(Vote vote, List<NoVoteReason> reasons) {
    public enum Vote { YES, NO }
    public static TransactionVote yes() { return new TransactionVote(Vote.YES, null); }
    public static TransactionVote no(List<NoVoteReason> reasons) {
        if (reasons == null || reasons.isEmpty()) throw new IllegalArgumentException("NO vote requires at least 1 reason");
        return new TransactionVote(Vote.NO, reasons);
    }
    @JsonIgnore
    public boolean isYes() { return vote == Vote.YES; }
}
