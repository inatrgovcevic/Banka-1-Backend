package com.banka1.interbank.protocol.dto;

import com.fasterxml.jackson.databind.ObjectMapper;
import org.junit.jupiter.api.Test;
import java.util.List;
import static org.junit.jupiter.api.Assertions.*;

class TransactionVoteTest {
    private final ObjectMapper mapper = new ObjectMapper();

    @Test void yesSerializesWithoutReasons() throws Exception {
        String json = mapper.writeValueAsString(TransactionVote.yes());
        assertEquals("{\"vote\":\"YES\"}", json);
    }

    @Test void noSerializesWithReasons() throws Exception {
        var vote = TransactionVote.no(List.of(NoVoteReason.of(NoVoteReason.Reason.UNBALANCED_TX)));
        String json = mapper.writeValueAsString(vote);
        assertTrue(json.contains("\"vote\":\"NO\""));
        assertTrue(json.contains("UNBALANCED_TX"));
    }

    @Test void noVoteRequiresAtLeastOneReason() {
        assertThrows(IllegalArgumentException.class, () -> TransactionVote.no(List.of()));
        assertThrows(IllegalArgumentException.class, () -> TransactionVote.no(null));
    }
}
