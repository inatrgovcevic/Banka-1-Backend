package com.banka1.interbank.client;

import static org.assertj.core.api.Assertions.assertThat;
import static org.springframework.test.web.client.match.MockRestRequestMatchers.jsonPath;
import static org.springframework.test.web.client.match.MockRestRequestMatchers.method;
import static org.springframework.test.web.client.match.MockRestRequestMatchers.requestTo;
import static org.springframework.test.web.client.response.MockRestResponseCreators.withNoContent;
import static org.springframework.test.web.client.response.MockRestResponseCreators.withSuccess;

import com.banka1.interbank.otc.dto.PublicStockEntryDto;
import java.util.List;
import java.util.UUID;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.springframework.http.HttpMethod;
import org.springframework.http.MediaType;
import org.springframework.test.web.client.MockRestServiceServer;
import org.springframework.web.client.RestClient;

/**
 * PR_32 Phase 5/15 unit testovi za {@link TradingInternalClient}.
 *
 * <p>Phase 15 update: paths usaglaseni sa Tim 2 §11 i Phase 12 controller-ima
 * ({@code /reservations/{id}}, {@code /options/{id}/(reserve|exercise|release)},
 * {@code /public-stocks}). {@code resolveAccount} je premešten u
 * {@link BankingCoreInternalClient} jer accounts žive u banking-core.
 */
class TradingInternalClientTest {

    private MockRestServiceServer mockServer;
    private TradingInternalClient client;

    @BeforeEach
    void setUp() {
        RestClient.Builder builder = RestClient.builder().baseUrl("http://trading:8088");
        mockServer = MockRestServiceServer.bindTo(builder).build();
        client = new TradingInternalClient(builder.build());
    }

    @Test
    void reserveStockPostsCorrectBody() {
        UUID reservationId = UUID.randomUUID();
        mockServer.expect(requestTo("http://trading:8088/internal/interbank/reserve-stock"))
                .andExpect(method(HttpMethod.POST))
                .andExpect(jsonPath("$.sellerUserId").value(42))
                .andExpect(jsonPath("$.ticker").value("AAPL"))
                .andExpect(jsonPath("$.quantity").value(10))
                .andExpect(jsonPath("$.transactionIdRouting").value(222))
                .andExpect(jsonPath("$.transactionIdLocal").value("tx-local-3"))
                .andRespond(withSuccess(
                        "{\"reservationId\":\"" + reservationId + "\"}",
                        MediaType.APPLICATION_JSON));

        TradingInternalClient.ReserveStockRes res = client.reserveStock(
                new TradingInternalClient.ReserveStockReq(42L, "AAPL", 10, 222, "tx-local-3"));

        assertThat(res.reservationId()).isEqualTo(reservationId);
        mockServer.verify();
    }

    @Test
    void commitStockPostsToCorrectUri() {
        UUID reservationId = UUID.fromString("00000000-0000-0000-0000-000000000010");
        mockServer.expect(requestTo(
                        "http://trading:8088/internal/interbank/reservations/"
                                + reservationId + "/commit-stock"))
                .andExpect(method(HttpMethod.POST))
                .andRespond(withNoContent());

        client.commitStock(reservationId);

        mockServer.verify();
    }

    @Test
    void releaseStockDeletesReservation() {
        UUID reservationId = UUID.fromString("00000000-0000-0000-0000-000000000011");
        mockServer.expect(requestTo(
                        "http://trading:8088/internal/interbank/reservations/" + reservationId))
                .andExpect(method(HttpMethod.DELETE))
                .andRespond(withNoContent());

        client.releaseStock(reservationId);

        mockServer.verify();
    }

    @Test
    void reserveOptionPostsExpectedBody() {
        mockServer.expect(requestTo("http://trading:8088/internal/interbank/options/neg-1/reserve"))
                .andExpect(method(HttpMethod.POST))
                .andExpect(jsonPath("$.sellerForeignId").value("seller-foreign-7"))
                .andExpect(jsonPath("$.ticker").value("MSFT"))
                .andExpect(jsonPath("$.quantity").value(5))
                .andRespond(withNoContent());

        client.reserveOption("neg-1", "seller-foreign-7", "MSFT", 5);

        mockServer.verify();
    }

    @Test
    void exerciseOptionPostsToCorrectUri() {
        mockServer.expect(requestTo("http://trading:8088/internal/interbank/options/neg-2/exercise"))
                .andExpect(method(HttpMethod.POST))
                .andRespond(withNoContent());

        client.exerciseOption("neg-2");

        mockServer.verify();
    }

    @Test
    void releaseOptionDeletesNegotiation() {
        mockServer.expect(requestTo("http://trading:8088/internal/interbank/options/neg-3/release"))
                .andExpect(method(HttpMethod.DELETE))
                .andRespond(withNoContent());

        client.releaseOption("neg-3");

        mockServer.verify();
    }

    @Test
    void getPublicStocksParsesArrayResponse() {
        mockServer.expect(requestTo("http://trading:8088/internal/interbank/public-stocks"))
                .andExpect(method(HttpMethod.GET))
                .andRespond(withSuccess(
                        "[{\"stock\":{\"ticker\":\"AAPL\"},\"sellers\":[{\"seller\":{\"routingNumber\":111,\"id\":\"sid-1\"},\"amount\":50}]}]",
                        MediaType.APPLICATION_JSON));

        List<PublicStockEntryDto> result = client.getPublicStocks();

        assertThat(result).hasSize(1);
        assertThat(result.get(0).stock().ticker()).isEqualTo("AAPL");
        assertThat(result.get(0).sellers()).hasSize(1);
        assertThat(result.get(0).sellers().get(0).amount()).isEqualTo(50);
        assertThat(result.get(0).sellers().get(0).seller().routingNumber()).isEqualTo(111);
        mockServer.verify();
    }
}
