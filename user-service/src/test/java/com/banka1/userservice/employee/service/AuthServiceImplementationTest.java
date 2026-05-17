package com.banka1.userservice.employee.service;

import com.banka1.employeeService.domain.Zaposlen;
import com.banka1.employeeService.dto.requests.LoginRequestDto;
import com.banka1.employeeService.exception.BusinessException;
import com.banka1.employeeService.repository.ZaposlenRepository;
import com.banka1.employeeService.security.JWTService;
import com.banka1.employeeService.service.implementation.AuthServiceImplementation;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.InjectMocks;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;
import org.springframework.security.crypto.password.PasswordEncoder;
import org.springframework.test.util.ReflectionTestUtils;

import java.time.LocalDateTime;
import java.util.Optional;

import static org.assertj.core.api.Assertions.assertThatThrownBy;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.argThat;
import static org.mockito.Mockito.atLeastOnce;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

/**
 * PR_32 Phase 13: imports re-targetovani sa nestaleg paketa
 * {@code com.banka1.userservice.employee.*} na konsolidovani
 * {@code com.banka1.employeeService.*}, i test logika usaglasena sa trenutnom
 * implementacijom {@link AuthServiceImplementation} (PR_30 Celina-1 lockout fix-ovi):
 *
 * <ul>
 *   <li>login flow koristi {@code findByEmail}, ne {@code findByUsername};</li>
 *   <li>spec (Celina 1 Sc 3) razlikuje {@code USER_NOT_FOUND} od {@code INVALID_CREDENTIALS};</li>
 *   <li>{@code accountLockoutMaxAttempts}/{@code accountLockoutDurationMinutes} se
 *       injectuju preko {@code @Value}; u testu ih postavljamo kroz {@code ReflectionTestUtils}
 *       jer {@code @InjectMocks} ne resava {@code @Value} placeholder-e.</li>
 * </ul>
 */
@ExtendWith(MockitoExtension.class)
class AuthServiceImplementationTest {

    @Mock private ZaposlenRepository zaposlenRepository;
    @Mock private PasswordEncoder passwordEncoder;
    @Mock private JWTService jwtService;

    @InjectMocks private AuthServiceImplementation service;

    @BeforeEach
    void setUp() {
        ReflectionTestUtils.setField(service, "accountLockoutMaxAttempts", 5);
        ReflectionTestUtils.setField(service, "accountLockoutDurationMinutes", 15L);
    }

    @Test
    void login_throws_kada_korisnik_ne_postoji() {
        when(zaposlenRepository.findByEmail("admin@banka.com")).thenReturn(Optional.empty());
        LoginRequestDto dto = new LoginRequestDto();
        dto.setEmail("admin@banka.com");
        dto.setPassword("admin123");

        assertThatThrownBy(() -> service.login(dto))
                .isInstanceOf(BusinessException.class);
    }

    @Test
    void login_throws_kada_pogresna_lozinka_inkrementira_failedAttempts() {
        Zaposlen z = new Zaposlen();
        z.setEmail("admin@banka.com");
        z.setPassword("hashed");
        z.setFailedLoginAttempts(0);
        z.setAktivan(true);

        when(zaposlenRepository.findByEmail("admin@banka.com")).thenReturn(Optional.of(z));
        when(passwordEncoder.matches(any(), any())).thenReturn(false);

        LoginRequestDto dto = new LoginRequestDto();
        dto.setEmail("admin@banka.com");
        dto.setPassword("wrong");

        assertThatThrownBy(() -> service.login(dto))
                .isInstanceOf(BusinessException.class);

        // Verifikacija: failedLoginAttempts mora biti inkrementiran u DB-u (Celina 1 Scenario 5)
        verify(zaposlenRepository, atLeastOnce()).save(argThat(saved ->
                saved.getFailedLoginAttempts() == 1));
    }

    @Test
    void login_zakljucava_nalog_posle_5_fail_a() {
        Zaposlen z = new Zaposlen();
        z.setEmail("admin@banka.com");
        z.setFailedLoginAttempts(4);
        z.setAktivan(true);

        when(zaposlenRepository.findByEmail("admin@banka.com")).thenReturn(Optional.of(z));
        when(passwordEncoder.matches(any(), any())).thenReturn(false);

        LoginRequestDto dto = new LoginRequestDto();
        dto.setEmail("admin@banka.com");
        dto.setPassword("wrong");

        assertThatThrownBy(() -> service.login(dto))
                .isInstanceOf(BusinessException.class);

        verify(zaposlenRepository).save(argThat(saved ->
                saved.getFailedLoginAttempts() >= 5 && saved.getLockedUntil() != null));
    }

    @Test
    void login_throws_kada_je_nalog_zakljucan_do_buduceg_vremena() {
        Zaposlen z = new Zaposlen();
        z.setEmail("admin@banka.com");
        z.setLockedUntil(LocalDateTime.now().plusMinutes(10));

        when(zaposlenRepository.findByEmail("admin@banka.com")).thenReturn(Optional.of(z));

        LoginRequestDto dto = new LoginRequestDto();
        dto.setEmail("admin@banka.com");
        dto.setPassword("admin123");

        assertThatThrownBy(() -> service.login(dto))
                .isInstanceOf(BusinessException.class);
    }
}
