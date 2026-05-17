package com.banka1.interbank.protocol.dto;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.DeserializationFeature;
import com.fasterxml.jackson.datatype.jsr310.JavaTimeModule;
import org.junit.jupiter.api.Test;
import java.math.BigDecimal;
import static org.junit.jupiter.api.Assertions.*;

class AssetTest {
    private final ObjectMapper mapper = new ObjectMapper()
        .registerModule(new JavaTimeModule())
        .configure(DeserializationFeature.USE_BIG_DECIMAL_FOR_FLOATS, true)
        .configure(com.fasterxml.jackson.databind.SerializationFeature.WRITE_DATES_AS_TIMESTAMPS, false);

    @Test void monasDeserialize() throws Exception {
        var json = "{\"type\":\"MONAS\",\"asset\":{\"currency\":\"USD\"}}";
        Asset a = mapper.readValue(json, Asset.class);
        assertInstanceOf(Asset.Monas.class, a);
        assertEquals(CurrencyCode.USD, ((Asset.Monas) a).asset().currency());
    }

    @Test void stockDeserialize() throws Exception {
        var json = "{\"type\":\"STOCK\",\"asset\":{\"ticker\":\"AAPL\"}}";
        Asset a = mapper.readValue(json, Asset.class);
        assertInstanceOf(Asset.Stock.class, a);
        assertEquals("AAPL", ((Asset.Stock) a).asset().ticker());
    }

    @Test void optionDeserialize() throws Exception {
        var json = """
            {"type":"OPTION","asset":{
              "negotiationId":{"routingNumber":222,"id":"N-17"},
              "stock":{"ticker":"AAPL"},
              "pricePerUnit":{"currency":"USD","amount":150},
              "settlementDate":"2026-06-15T00:00:00+02:00",
              "amount":100
            }}""";
        Asset a = mapper.readValue(json, Asset.class);
        assertInstanceOf(Asset.Option.class, a);
        var opt = (Asset.Option) a;
        assertEquals(100, opt.asset().amount());
        assertEquals(0, opt.asset().pricePerUnit().amount().compareTo(new BigDecimal(150)));
    }
}
