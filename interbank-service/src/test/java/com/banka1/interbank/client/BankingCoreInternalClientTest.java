package com.banka1.interbank.client;

import static org.assertj.core.api.Assertions.assertThat;
import static org.springframework.test.web.client.match.MockRestRequestMatchers.content;
import static org.springframework.test.web.client.match.MockRestRequestMatchers.jsonPath;
import static org.springframework.test.web.client.match.MockRestRequestMatchers.method;
import static org.springframework.test.web.client.match.MockRestRequestMatchers.requestTo;
import static org.springframework.test.web.client.response.MockRestResponseCreators.withNoContent;
import static org.springframework.test.web.client.response.MockRestResponseCreators.withSuccess;

import com.banka1.interbank.protocol.dto.CurrencyCode;
import java.math.BigDecimal;
import java.util.UUID;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.springframework.http.HttpMethod;
import org.springframework.http.MediaType;
import org.springframework.test.web.client.MockRestServiceServer;
import org.springframework.web.client.RestClient;

/**
 * PR_32 Phase 5 unit testovi za {@link BankingCoreInternalClient}.
 *
 * <p>Koristimo {@link MockRestServiceServer} bound za internu {@code RestClient}
 * instancu — istestiramo da klijent formira korektan HTTP request (URL, body,
 * metod) bez stvaranja Spring konteksta.
 */
class BankingCoreInternalClientTest {

    private MockRestServiceServer mockServer;
    private BankingCoreInternalClient client;

    @BeforeEach
    void setUp() {
        RestClient.Builder builder = RestClient.builder().baseUrl("http://banking-core:8084");
        mockServer = MockRestServiceServer.bindTo(builder).build();
        client = new BankingCoreInternalClient(builder.build());
    }

    @Test
    void reserveMonasPostsToCorrectUriWithJsonBody() {
        UUID reservationId = UUID.randomUUID();
        mockServer.expect(requestTo("http://banking-core:8084/internal/interbank/reserve-monas"))
                .andExpect(method(HttpMethod.POST))
                .andExpect(jsonPath("$.accountNum").value("111000001234567890"))
                .andExpect(jsonPath("$.currency").value("EUR"))
                .andExpect(jsonPath("$.amount").value(100.50))
                .andExpect(jsonPath("$.transactionIdRouting").value(222))
                .andExpect(jsonPath("$.transactionIdLocal").value("tx-local-1"))
                .andRespond(withSuccess(
                        "{\"reservationId\":\"" + reservationId + "\"}",
                        MediaType.APPLICATION_JSON));

        BankingCoreInternalClient.ReserveMonasReq req = new BankingCoreInternalClient.ReserveMonasReq(
                "111000001234567890", CurrencyCode.EUR, new BigDecimal("100.50"), 222, "tx-local-1");

        BankingCoreInternalClient.ReserveMonasRes res = client.reserveMonas(req);

        assertThat(res.reservationId()).isEqualTo(reservationId);
        mockServer.verify();
    }

    @Test
    void commitMonasPostsToReservationCommitUri() {
        UUID reservationId = UUID.fromString("00000000-0000-0000-0000-000000000001");
        mockServer.expect(requestTo(
                        "http://banking-core:8084/internal/interbank/reservations/"
                                + reservationId + "/commit-monas"))
                .andExpect(method(HttpMethod.POST))
                .andRespond(withNoContent());

        client.commitMonas(reservationId);

        mockServer.verify();
    }

    @Test
    void releaseMonasDeletesReservation() {
        UUID reservationId = UUID.fromString("00000000-0000-0000-0000-000000000002");
        mockServer.expect(requestTo(
                        "http://banking-core:8084/internal/interbank/reservations/" + reservationId))
                .andExpect(method(HttpMethod.DELETE))
                .andRespond(withNoContent());

        client.releaseMonas(reservationId);

        mockServer.verify();
    }

    @Test
    void resolveAccountGetsAndParsesResponse() {
        mockServer.expect(requestTo(
                        "http://banking-core:8084/internal/interbank/account-resolve?num=111000001234567890"))
                .andExpect(method(HttpMethod.GET))
                .andRespond(withSuccess(
                        "{\"ownerType\":\"CLIENT\",\"ownerId\":42,\"currency\":\"EUR\",\"availableBalance\":1234.56}",
                        MediaType.APPLICATION_JSON));

        BankingCoreInternalClient.AccountResolveRes res =
                client.resolveAccount("111000001234567890");

        assertThat(res.ownerType()).isEqualTo("CLIENT");
        assertThat(res.ownerId()).isEqualTo(42L);
        assertThat(res.currency()).isEqualTo(CurrencyCode.EUR);
        assertThat(res.availableBalance()).isEqualByComparingTo(new BigDecimal("1234.56"));
        mockServer.verify();
    }

    @Test
    void reserveMonasSerializesBigDecimalWithoutLoss() {
        UUID reservationId = UUID.randomUUID();
        // 19-digit precision past double range — verifies BigDecimal-as-string path.
        BigDecimal preciseAmount = new BigDecimal("1234567890.1234567890");

        mockServer.expect(requestTo("http://banking-core:8084/internal/interbank/reserve-monas"))
                .andExpect(method(HttpMethod.POST))
                .andExpect(content().string(org.hamcrest.Matchers.containsString("1234567890.1234567890")))
                .andRespond(withSuccess(
                        "{\"reservationId\":\"" + reservationId + "\"}",
                        MediaType.APPLICATION_JSON));

        client.reserveMonas(new BankingCoreInternalClient.ReserveMonasReq(
                "111000001111111111", CurrencyCode.RSD, preciseAmount, 111, "tx-local-2"));

        mockServer.verify();
    }
}
