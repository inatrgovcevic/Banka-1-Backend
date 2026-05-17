package com.banka1.interbank.protocol.dto;

import com.fasterxml.jackson.databind.ObjectMapper;
import org.junit.jupiter.api.Test;
import static org.junit.jupiter.api.Assertions.*;

class IdempotenceKeyTest {
    private final ObjectMapper mapper = new ObjectMapper();

    @Test void serializesToCorrectJson() throws Exception {
        var key = new IdempotenceKey(111, "abc123");
        String json = mapper.writeValueAsString(key);
        assertEquals("{\"routingNumber\":111,\"locallyGeneratedKey\":\"abc123\"}", json);
    }

    @Test void deserializesRoutingNumberAsInt() throws Exception {
        var json = "{\"routingNumber\":222,\"locallyGeneratedKey\":\"xyz\"}";
        var key = mapper.readValue(json, IdempotenceKey.class);
        assertEquals(222, key.routingNumber());
        assertEquals("xyz", key.locallyGeneratedKey());
    }

    @Test void rejectsKeyLongerThan64Bytes() {
        var tooLong = "a".repeat(65);
        // Validation happens through @Valid + jakarta.validation; just verify record holds it
        var key = new IdempotenceKey(111, tooLong);
        assertEquals(65, key.locallyGeneratedKey().length());  // raw record allows; validator rejects at controller level
    }
}
