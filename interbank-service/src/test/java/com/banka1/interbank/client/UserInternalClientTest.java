package com.banka1.interbank.client;

import static org.assertj.core.api.Assertions.assertThat;
import static org.springframework.test.web.client.match.MockRestRequestMatchers.method;
import static org.springframework.test.web.client.match.MockRestRequestMatchers.requestTo;
import static org.springframework.test.web.client.response.MockRestResponseCreators.withSuccess;

import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.springframework.http.HttpMethod;
import org.springframework.http.MediaType;
import org.springframework.test.web.client.MockRestServiceServer;
import org.springframework.web.client.RestClient;

/**
 * PR_32 Phase 5 unit testovi za {@link UserInternalClient}.
 */
class UserInternalClientTest {

    private MockRestServiceServer mockServer;
    private UserInternalClient client;

    @BeforeEach
    void setUp() {
        RestClient.Builder builder = RestClient.builder().baseUrl("http://user:8081");
        mockServer = MockRestServiceServer.bindTo(builder).build();
        client = new UserInternalClient(builder.build());
    }

    @Test
    void resolveUserGetsAndParsesResponseForClient() {
        mockServer.expect(requestTo("http://user:8081/internal/interbank/user/CLIENT/42"))
                .andExpect(method(HttpMethod.GET))
                .andRespond(withSuccess(
                        "{\"firstName\":\"Marko\",\"lastName\":\"Markovic\",\"fullName\":\"Marko Markovic\"}",
                        MediaType.APPLICATION_JSON));

        UserInternalClient.UserDisplayRes res = client.resolveUser("client", 42L);

        assertThat(res.firstName()).isEqualTo("Marko");
        assertThat(res.lastName()).isEqualTo("Markovic");
        assertThat(res.fullName()).isEqualTo("Marko Markovic");
        mockServer.verify();
    }

    @Test
    void resolveUserUppercasesType() {
        mockServer.expect(requestTo("http://user:8081/internal/interbank/user/EMPLOYEE/7"))
                .andExpect(method(HttpMethod.GET))
                .andRespond(withSuccess(
                        "{\"firstName\":\"Ana\",\"lastName\":\"Anic\",\"fullName\":\"Ana Anic\"}",
                        MediaType.APPLICATION_JSON));

        // Lower-case input — implementacija mora da uradi uppercase pre nego sto
        // ubaci u URI.
        UserInternalClient.UserDisplayRes res = client.resolveUser("employee", 7L);

        assertThat(res.fullName()).isEqualTo("Ana Anic");
        mockServer.verify();
    }
}
