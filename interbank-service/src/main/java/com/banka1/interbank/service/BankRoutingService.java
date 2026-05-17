package com.banka1.interbank.service;

import com.banka1.interbank.config.InterbankProperties;
import java.util.List;
import java.util.Set;
import java.util.stream.Collectors;
import lombok.RequiredArgsConstructor;
import org.springframework.stereotype.Service;

/**
 * PR_32 Phase 8 Task 8.1: helper za rezolvanje partner banaka po routing brojevima.
 *
 * <p>Koristi se kao tanak wrapper nad {@link InterbankProperties} u biznis kodu da
 * koordinator transakcije ne zavisi direktno od konfiguracionog DTO-a. Sve metode su
 * cisti delegati nad konfiguracijom — bez state-a.
 *
 * <ul>
 *   <li>{@link #resolvePartnerByRouting(int)} — strict lookup partnera (baca
 *       {@link IllegalArgumentException} ako routing nije konfigurisan).</li>
 *   <li>{@link #distinctPartnerRoutings(List)} — iz liste routing brojeva izvuce
 *       distinct set partnerskih routing brojeva (preskoci nas).</li>
 *   <li>{@link #isMine(int)} — true ako je routing broj nas (= local bank).</li>
 * </ul>
 */
@Service
@RequiredArgsConstructor
public class BankRoutingService {

    private final InterbankProperties props;

    /**
     * Strict lookup partnera po routing broju.
     *
     * @param rn routing number trazenog partnera
     * @return Partner objekat iz konfiguracije
     * @throws IllegalArgumentException ako routing nije konfigurisan u
     *                                  {@code interbank.partners[*]}
     */
    public InterbankProperties.Partner resolvePartnerByRouting(int rn) {
        return props.partnerOrThrow(rn);
    }

    /**
     * Vrati distinct set routing brojeva koji NISU nasi (preskoci local routing).
     *
     * <p>Koristi se za fan-out u koordinatoru — iz svih posting routing brojeva
     * izvlacimo samo distinct partnere koje treba kontaktirati.
     *
     * @param routings lista routing brojeva (moze sadrzati duplikate i nase)
     * @return set distinct partner routing brojeva (bez naseg)
     */
    public Set<Integer> distinctPartnerRoutings(List<Integer> routings) {
        return routings.stream()
                .filter(r -> r != props.getMyRoutingNumber())
                .collect(Collectors.toSet());
    }

    /**
     * Da li je {@code rn} nas routing broj?
     */
    public boolean isMine(int rn) {
        return rn == props.getMyRoutingNumber();
    }
}
