package com.banka1.interbank.config;

import com.banka1.interbank.auth.InterbankAuthFilter;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import org.springframework.core.annotation.Order;
import org.springframework.security.config.annotation.web.builders.HttpSecurity;
import org.springframework.security.config.annotation.web.configurers.AbstractHttpConfigurer;
import org.springframework.security.config.http.SessionCreationPolicy;
import org.springframework.security.web.SecurityFilterChain;
import org.springframework.security.web.authentication.UsernamePasswordAuthenticationFilter;

/**
 * PR_32 Phase 4: dedikovani SecurityFilterChain (@Order(1)) za inter-bank rute.
 *
 * <p>Ovaj chain hvata samo {@code /interbank}, {@code /public-stock} i
 * {@code /negotiations/**} rute i odbija ih osim ako ne dolaze sa validnim
 * {@code X-Api-Key} header-om kome je {@link InterbankAuthFilter} dodao
 * {@code ROLE_INTERBANK_PARTNER}.
 *
 * <p>{@code @Order(1)} → ide ISPRED security-lib {@code authChain} (@Order(1))
 * i {@code apiChain} (@Order(2)). NAPOMENA: security-lib koristi
 * {@code @Order(1)} za permit-all rute (Swagger, actuator/health/liveness,
 * itd.). Posto su nasi {@code securityMatcher} prefiksi disjunktivni od
 * permit-all liste, ne dolazi do kolizije: Spring bira chain po prvom matchu
 * {@code securityMatcher}-a. Ipak, sa istim {@code @Order(1)} order izmedju njih
 * je neodredjen — koristimo {@code @Order(0)} da budemo sigurni da nas chain
 * dobije priliku prvi.
 *
 * <p>STATELESS session, CSRF disabled (inter-bank pozivi su server-to-server,
 * cookie auth ne postoji). CORS isto disabled jer browser ne treba da pristupa
 * ovim rutama direktno.
 */
@Configuration
public class InterbankSecurityConfig {

    @Bean
    public InterbankAuthFilter interbankAuthFilter(InterbankProperties properties) {
        return new InterbankAuthFilter(properties);
    }

    @Bean
    @Order(0)
    public SecurityFilterChain interbankChain(HttpSecurity http,
                                              InterbankAuthFilter filter) throws Exception {
        http
                .securityMatcher(
                        "/interbank",
                        "/interbank/**",
                        "/public-stock",
                        "/public-stock/**",
                        "/negotiations",
                        "/negotiations/**"
                )
                .csrf(AbstractHttpConfigurer::disable)
                .cors(AbstractHttpConfigurer::disable)
                .sessionManagement(s -> s.sessionCreationPolicy(SessionCreationPolicy.STATELESS))
                .authorizeHttpRequests(auth -> auth.anyRequest().hasRole("INTERBANK_PARTNER"))
                .addFilterBefore(filter, UsernamePasswordAuthenticationFilter.class);

        return http.build();
    }
}
