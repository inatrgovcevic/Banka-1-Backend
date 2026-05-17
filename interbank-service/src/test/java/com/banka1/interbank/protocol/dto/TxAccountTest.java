package com.banka1.interbank.protocol.dto;

import com.fasterxml.jackson.databind.ObjectMapper;
import org.junit.jupiter.api.Test;
import static org.junit.jupiter.api.Assertions.*;

class TxAccountTest {
    private final ObjectMapper mapper = new ObjectMapper();

    @Test void personDeserializesByTypeField() throws Exception {
        var json = "{\"type\":\"PERSON\",\"id\":{\"routingNumber\":111,\"id\":\"C-5\"}}";
        TxAccount acc = mapper.readValue(json, TxAccount.class);
        assertInstanceOf(TxAccount.Person.class, acc);
        assertEquals("C-5", ((TxAccount.Person) acc).id().id());
    }

    @Test void accountDeserializesByTypeField() throws Exception {
        var json = "{\"type\":\"ACCOUNT\",\"num\":\"111000112345678901\"}";
        TxAccount acc = mapper.readValue(json, TxAccount.class);
        assertInstanceOf(TxAccount.Account.class, acc);
        assertEquals("111000112345678901", ((TxAccount.Account) acc).num());
    }

    @Test void optionDeserializesByTypeField() throws Exception {
        var json = "{\"type\":\"OPTION\",\"id\":{\"routingNumber\":222,\"id\":\"N-17\"}}";
        TxAccount acc = mapper.readValue(json, TxAccount.class);
        assertInstanceOf(TxAccount.Option.class, acc);
    }

    @Test void serializesWithTypeField() throws Exception {
        TxAccount acc = new TxAccount.Account("111000112345678901");
        String json = mapper.writeValueAsString(acc);
        assertTrue(json.contains("\"type\":\"ACCOUNT\""));
        assertTrue(json.contains("\"num\":\"111000112345678901\""));
    }
}
