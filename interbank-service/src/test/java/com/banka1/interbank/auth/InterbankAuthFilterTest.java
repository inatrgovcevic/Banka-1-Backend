package com.banka1.interbank.auth;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertNotNull;
import static org.junit.jupiter.api.Assertions.assertNull;
import static org.junit.jupiter.api.Assertions.assertTrue;
import static org.mockito.Mockito.never;
import static org.mockito.Mockito.times;
import static org.mockito.Mockito.verify;

import com.banka1.interbank.config.InterbankProperties;
import jakarta.servlet.FilterChain;
import jakarta.servlet.http.HttpServletResponse;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.mockito.Mockito;
import org.springframework.mock.web.MockFilterChain;
import org.springframework.mock.web.MockHttpServletRequest;
import org.springframework.mock.web.MockHttpServletResponse;
import org.springframework.security.core.Authentication;
import org.springframework.security.core.context.SecurityContextHolder;

/**
 * PR_32 Phase 4: unit testovi za {@link InterbankAuthFilter}.
 *
 * <p>Koristimo {@link MockHttpServletRequest} / {@link MockHttpServletResponse}
 * iz spring-test umesto Mockito-stubova HttpServletRequest jer
 * {@link org.springframework.web.filter.OncePerRequestFilter} ima ozbiljan
 * setup koji se brzo lomi (ALREADY_FILTERED_SUFFIX, attribute lookup, itd.).
 */
class InterbankAuthFilterTest {

    private InterbankProperties properties;
    private InterbankAuthFilter filter;

    @BeforeEach
    void setUp() {
        InterbankProperties.Partner partner = new InterbankProperties.Partner();
        partner.setRoutingNumber(222);
        partner.setDisplayName("Banka 2");
        partner.setBaseUrl("http://banka2.local/");
        partner.setInboundToken("valid-token-222");
        partner.setOutboundToken("out-222");

        properties = new InterbankProperties();
        properties.setMyRoutingNumber(111);
        properties.setMyBankDisplayName("Banka 1");
        properties.setPartners(java.util.List.of(partner));

        filter = new InterbankAuthFilter(properties);
        SecurityContextHolder.clearContext();
    }

    @AfterEach
    void tearDown() {
        SecurityContextHolder.clearContext();
    }

    @Test
    void rejectsMissingToken() throws Exception {
        MockHttpServletRequest req = new MockHttpServletRequest("POST", "/interbank/inbound");
        MockHttpServletResponse res = new MockHttpServletResponse();
        FilterChain chain = Mockito.mock(FilterChain.class);

        filter.doFilter(req, res, chain);

        assertEquals(HttpServletResponse.SC_UNAUTHORIZED, res.getStatus());
        verify(chain, never()).doFilter(Mockito.any(), Mockito.any());
        assertNull(SecurityContextHolder.getContext().getAuthentication(),
                "auth context mora ostati prazan kad token nedostaje");
    }

    @Test
    void rejectsInvalidToken() throws Exception {
        MockHttpServletRequest req = new MockHttpServletRequest("POST", "/negotiations/quote");
        req.addHeader("X-Api-Key", "wrong-token");
        MockHttpServletResponse res = new MockHttpServletResponse();
        FilterChain chain = Mockito.mock(FilterChain.class);

        filter.doFilter(req, res, chain);

        assertEquals(HttpServletResponse.SC_UNAUTHORIZED, res.getStatus());
        verify(chain, never()).doFilter(Mockito.any(), Mockito.any());
        assertNull(SecurityContextHolder.getContext().getAuthentication());
    }

    @Test
    void acceptsValidToken() throws Exception {
        MockHttpServletRequest req = new MockHttpServletRequest("POST", "/interbank/inbound");
        req.addHeader("X-Api-Key", "valid-token-222");
        MockHttpServletResponse res = new MockHttpServletResponse();
        MockFilterChain chain = new MockFilterChain();

        filter.doFilter(req, res, chain);

        assertEquals(HttpServletResponse.SC_OK, res.getStatus(),
                "valid token treba da propusti zahtev dalje (default status 200 do controllera)");
        Authentication auth = SecurityContextHolder.getContext().getAuthentication();
        assertNotNull(auth, "auth context mora biti popunjen");
        assertTrue(auth.isAuthenticated());
        assertTrue(auth.getAuthorities().stream()
                        .anyMatch(a -> "ROLE_INTERBANK_PARTNER".equals(a.getAuthority())),
                "mora imati ROLE_INTERBANK_PARTNER authority");
        assertEquals(InterbankProperties.Partner.class, auth.getPrincipal().getClass());
        assertEquals(222, ((InterbankProperties.Partner) auth.getPrincipal()).getRoutingNumber());
    }

    @Test
    void skipsUnprotectedPaths() throws Exception {
        // Pat 1: ne-protected ruta uopste (npr. /accounts/123). Bez X-Api-Key.
        MockHttpServletRequest req1 = new MockHttpServletRequest("GET", "/accounts/123");
        MockHttpServletResponse res1 = new MockHttpServletResponse();
        FilterChain chain1 = Mockito.mock(FilterChain.class);

        filter.doFilter(req1, res1, chain1);

        assertEquals(HttpServletResponse.SC_OK, res1.getStatus());
        verify(chain1, times(1)).doFilter(Mockito.any(), Mockito.any());
        assertNull(SecurityContextHolder.getContext().getAuthentication());

        // Pat 2: /actuator/health/* prefiks (eksplicitno preskocen).
        MockHttpServletRequest req2 = new MockHttpServletRequest("GET", "/actuator/health/liveness");
        MockHttpServletResponse res2 = new MockHttpServletResponse();
        FilterChain chain2 = Mockito.mock(FilterChain.class);

        filter.doFilter(req2, res2, chain2);

        assertEquals(HttpServletResponse.SC_OK, res2.getStatus());
        verify(chain2, times(1)).doFilter(Mockito.any(), Mockito.any());
    }
}
