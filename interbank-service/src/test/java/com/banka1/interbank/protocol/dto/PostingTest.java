package com.banka1.interbank.protocol.dto;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.DeserializationFeature;
import org.junit.jupiter.api.Test;
import java.math.BigDecimal;
import static org.junit.jupiter.api.Assertions.*;

class PostingTest {
    private final ObjectMapper mapper = new ObjectMapper()
        .configure(DeserializationFeature.USE_BIG_DECIMAL_FOR_FLOATS, true);

    @Test void negativeAmountIsCredit() throws Exception {
        var json = "{\"account\":{\"type\":\"ACCOUNT\",\"num\":\"111000112345678901\"},\"amount\":-1500.25,\"asset\":{\"type\":\"MONAS\",\"asset\":{\"currency\":\"USD\"}}}";
        Posting p = mapper.readValue(json, Posting.class);
        assertEquals(-1, p.amount().signum());
        assertEquals(0, p.amount().compareTo(new BigDecimal("-1500.25")));
    }

    @Test void positiveAmountIsDebit() throws Exception {
        var json = "{\"account\":{\"type\":\"ACCOUNT\",\"num\":\"222000112345678902\"},\"amount\":1500,\"asset\":{\"type\":\"MONAS\",\"asset\":{\"currency\":\"USD\"}}}";
        Posting p = mapper.readValue(json, Posting.class);
        assertEquals(1, p.amount().signum());
    }
}
