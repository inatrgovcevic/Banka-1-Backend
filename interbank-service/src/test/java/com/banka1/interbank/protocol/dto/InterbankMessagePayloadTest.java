package com.banka1.interbank.protocol.dto;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.DeserializationFeature;
import org.junit.jupiter.api.Test;
import static org.junit.jupiter.api.Assertions.*;

class InterbankMessagePayloadTest {
    private final ObjectMapper mapper = new ObjectMapper()
        .configure(DeserializationFeature.USE_BIG_DECIMAL_FOR_FLOATS, true);

    @Test void newTxEnvelope() throws Exception {
        var json = """
            {"idempotenceKey":{"routingNumber":111,"locallyGeneratedKey":"k1"},
             "messageType":"NEW_TX",
             "message":{"postings":[],"transactionId":{"routingNumber":111,"id":"t1"},"paymentCode":"289","paymentPurpose":"X"}}""";
        var msg = mapper.readValue(json, InterbankMessagePayload.class);
        assertEquals(MessageType.NEW_TX, msg.messageType());
        assertEquals(111, msg.idempotenceKey().routingNumber());
        assertNotNull(msg.message());
    }

    @Test void commitTxEnvelopeNestedPayload() throws Exception {
        var json = """
            {"idempotenceKey":{"routingNumber":222,"locallyGeneratedKey":"k2"},
             "messageType":"COMMIT_TX",
             "message":{"transactionId":{"routingNumber":111,"id":"t1"}}}""";
        var msg = mapper.readValue(json, InterbankMessagePayload.class);
        assertEquals(MessageType.COMMIT_TX, msg.messageType());
        var body = mapper.treeToValue(msg.message(), CommitTransactionBody.class);
        assertEquals("t1", body.transactionId().id());
    }
}
