package com.banka1.interbank.protocol.dto;

import com.fasterxml.jackson.databind.ObjectMapper;
import org.junit.jupiter.api.Test;
import static org.junit.jupiter.api.Assertions.*;

class ForeignBankIdTest {
    private final ObjectMapper mapper = new ObjectMapper();

    @Test void roundTripJson() throws Exception {
        var id = new ForeignBankId(111, "C-5");
        String json = mapper.writeValueAsString(id);
        var parsed = mapper.readValue(json, ForeignBankId.class);
        assertEquals(id, parsed);
    }
}
