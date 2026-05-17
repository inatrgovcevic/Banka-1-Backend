package com.banka1.interbank.protocol.dto;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.DeserializationFeature;
import org.junit.jupiter.api.Test;
import java.math.BigDecimal;
import static org.junit.jupiter.api.Assertions.*;

class MonetaryValueTest {
    private final ObjectMapper mapper = new ObjectMapper()
        .configure(DeserializationFeature.USE_BIG_DECIMAL_FOR_FLOATS, true);

    @Test void preservesBigDecimalPrecision() throws Exception {
        var v = new MonetaryValue(CurrencyCode.USD, new BigDecimal("1500.2500"));
        String json = mapper.writeValueAsString(v);
        var parsed = mapper.readValue(json, MonetaryValue.class);
        assertEquals(0, parsed.amount().compareTo(new BigDecimal("1500.25")));
        assertEquals(BigDecimal.class, parsed.amount().getClass());
    }

    @Test void deserializesJsonNumberAsBigDecimal() throws Exception {
        var json = "{\"currency\":\"EUR\",\"amount\":150.7777}";
        var v = mapper.readValue(json, MonetaryValue.class);
        assertEquals(CurrencyCode.EUR, v.currency());
        assertEquals(new BigDecimal("150.7777"), v.amount());
    }
}
