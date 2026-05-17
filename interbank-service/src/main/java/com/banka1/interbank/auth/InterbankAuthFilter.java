package com.banka1.interbank.auth;

import com.banka1.interbank.config.InterbankProperties;
import jakarta.servlet.FilterChain;
import jakarta.servlet.ServletException;
import jakarta.servlet.http.HttpServletRequest;
import jakarta.servlet.http.HttpServletResponse;
import java.io.IOException;
import java.util.List;
import java.util.Optional;
import org.springframework.security.core.authority.SimpleGrantedAuthority;
import org.springframework.security.core.context.SecurityContextHolder;
import org.springframework.web.filter.OncePerRequestFilter;

/**
 * PR_32 Phase 4: filter koji autentifikuje INBOUND inter-bank pozive na osnovu
 * {@code X-Api-Key} header-a.
 *
 * <p>Obradjuje samo rute koje su definisane kao "inter-bank protokol prefiks":
 * {@code /interbank}, {@code /public-stock}, {@code /negotiations}. Sve ostalo
 * propusta dalje u chain (gde ce ga preuzeti security-lib JWT filter chain).
 *
 * <p>{@code /actuator/health/*} prefiks se eksplicitno preskace iako nije pod
 * gornjim prefikse-ima — health probe-i moraju da prolaze bez authentifikacije
 * cak i ako neko slucajno doda /actuator/health/interbank rutu.
 *
 * <p>Vraca {@code 401 Unauthorized} ako header nedostaje ili ne matchuje
 * nijednog konfigurisanog partnera. Ako matchuje, kreira
 * {@link InterbankAuthenticationToken} sa role {@code ROLE_INTERBANK_PARTNER} i
 * stavlja ga u {@link SecurityContextHolder}.
 */
public class InterbankAuthFilter extends OncePerRequestFilter {

    static final String API_KEY_HEADER = "X-Api-Key";
    static final String INTERBANK_ROLE = "ROLE_INTERBANK_PARTNER";

    private static final String[] PROTECTED_PREFIXES = {
        "/interbank",
        "/public-stock",
        "/negotiations"
    };

    private static final String ACTUATOR_HEALTH_PREFIX = "/actuator/health";

    private final InterbankProperties properties;

    public InterbankAuthFilter(InterbankProperties properties) {
        this.properties = properties;
    }

    @Override
    protected void doFilterInternal(HttpServletRequest request,
                                    HttpServletResponse response,
                                    FilterChain chain) throws ServletException, IOException {
        String uri = request.getRequestURI();

        if (uri == null || uri.startsWith(ACTUATOR_HEALTH_PREFIX) || !isProtected(uri)) {
            chain.doFilter(request, response);
            return;
        }

        String token = request.getHeader(API_KEY_HEADER);
        if (token == null || token.isBlank()) {
            response.setStatus(HttpServletResponse.SC_UNAUTHORIZED);
            return;
        }

        Optional<InterbankProperties.Partner> match = properties.findByInboundToken(token);
        if (match.isEmpty()) {
            response.setStatus(HttpServletResponse.SC_UNAUTHORIZED);
            return;
        }

        InterbankAuthenticationToken auth = new InterbankAuthenticationToken(
                match.get(),
                List.of(new SimpleGrantedAuthority(INTERBANK_ROLE))
        );
        SecurityContextHolder.getContext().setAuthentication(auth);
        chain.doFilter(request, response);
    }

    private static boolean isProtected(String uri) {
        for (String prefix : PROTECTED_PREFIXES) {
            if (uri.equals(prefix) || uri.startsWith(prefix + "/") || uri.startsWith(prefix + "?")) {
                return true;
            }
        }
        return false;
    }
}
