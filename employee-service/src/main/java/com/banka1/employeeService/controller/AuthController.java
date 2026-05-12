package com.banka1.employeeService.controller;

import com.banka1.employeeService.dto.requests.ActivateDto;
import com.banka1.employeeService.dto.requests.ForgotPasswordDto;
import com.banka1.employeeService.dto.requests.LoginRequestDto;
import com.banka1.employeeService.dto.requests.LogoutRequestDto;
import com.banka1.employeeService.dto.requests.RefreshTokenRequestDto;
import com.banka1.employeeService.dto.requests.ResendActivationDto;
import com.banka1.employeeService.dto.responses.TokenResponseDto;
import com.banka1.employeeService.service.AuthService;
import jakarta.validation.Valid;
import lombok.AllArgsConstructor;
import org.springframework.http.HttpStatus;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.*;

/**
 * REST kontroler koji izlaze endpoint-e za autentifikaciju i upravljanje nalozima.
 * Svi endpoint-i su dostupni pod baznom putanjom {@code /auth}.
 */
@RestController("employeeAuthController")
@RequestMapping("/employees/auth")
@AllArgsConstructor
public class AuthController {

    /** Servis koji sadrzi poslovnu logiku autentifikacije. */
    private AuthService authService;

    /**
     * Autentifikuje korisnika i vraca pristupni i refresh token.
     *
     * @param loginDto kredencijali za prijavu
     * @return odgovor sa generisanim tokenima
     */
    @PostMapping("/login")
    public ResponseEntity<TokenResponseDto> login(@RequestBody @Valid LoginRequestDto loginDto) {
        return new ResponseEntity<>(authService.login(loginDto), HttpStatus.OK);
    }

    /**
     * Rotira refresh token i vraca novi par tokena.
     *
     * @param refreshToken zahtev sa postojecim refresh tokenom
     * @return odgovor sa novim tokenima
     */
    @PostMapping("/refresh")
    public ResponseEntity<TokenResponseDto> refresh(@RequestBody @Valid RefreshTokenRequestDto refreshToken) {
        return new ResponseEntity<>(authService.refreshToken(refreshToken), HttpStatus.OK);
    }

    /**
     * Proverava validnost aktivacionog tokena dobijenog iz aktivacionog linka.
     *
     * @param confirmationToken token dobijen iz aktivacionog linka
     * @return identifikator potvrde ako je token validan
     */
    @GetMapping("/checkActivate")
    public ResponseEntity<Long> checkActivate(@RequestParam String confirmationToken) {
        return new ResponseEntity<>(authService.check(confirmationToken), HttpStatus.OK);
    }

    /**
     * Aktivira nalog i postavlja novu lozinku za korisnika.
     *
     * @param activateDto podaci potrebni za aktivaciju naloga
     * @return poruka o rezultatu aktivacije
     */
    @PostMapping("/activate")
    public ResponseEntity<String> activate(@RequestBody @Valid ActivateDto activateDto) {
        return new ResponseEntity<>(authService.editPassword(activateDto, true), HttpStatus.OK);
    }

    /**
     * Pokrece proces resetovanja lozinke slanjem mejla sa reset linkom.
     *
     * @param forgotPasswordDto zahtev sa email adresom korisnika
     * @return poruka o rezultatu slanja mejla
     */
    @PostMapping("/forgot-password")
    public ResponseEntity<String> forgotPassword(@RequestBody @Valid ForgotPasswordDto forgotPasswordDto) {
        return new ResponseEntity<>(authService.forgotPassword(forgotPasswordDto), HttpStatus.ACCEPTED);
    }

    /**
     * Proverava validnost tokena za reset lozinke dobijenog iz reset linka.
     *
     * @param confirmationToken token dobijen iz reset linka
     * @return identifikator potvrde ako je token validan
     */
    @GetMapping("/checkResetPassword")
    public ResponseEntity<Long> checkResetPassword(@RequestParam String confirmationToken) {
        return new ResponseEntity<>(authService.check(confirmationToken), HttpStatus.OK);
    }

    /**
     * Menja lozinku korisnika na osnovu validnog reset tokena.
     *
     * @param activateDto podaci potrebni za promenu lozinke
     * @return poruka o rezultatu promene lozinke
     */
    @PostMapping("/resetPassword")
    public ResponseEntity<String> resetPassword(@RequestBody @Valid ActivateDto activateDto) {
        return new ResponseEntity<>(authService.editPassword(activateDto, false), HttpStatus.OK);
    }

    /**
     * Odjavljuje korisnika brisanjem refresh tokena iz baze.
     *
     * @param logoutRequestDto zahtev sa refresh tokenom koji treba obrisati
     * @return prazan odgovor sa statusom 204
     */
    @DeleteMapping("/logout")
    public ResponseEntity<Void> logout(@RequestBody @Valid LogoutRequestDto logoutRequestDto) {
        authService.logout(logoutRequestDto.getRefreshToken());
        return new ResponseEntity<>(HttpStatus.NO_CONTENT);
    }

    /**
     * Ponovo salje aktivacioni mejl korisniku koji jos nije aktivirao nalog.
     *
     * @param resendActivationDto zahtev sa email adresom korisnika
     * @return poruka o rezultatu operacije
     */
    @PostMapping("/resend-activation")
    public ResponseEntity<String> resendActivation(@RequestBody @Valid ResendActivationDto resendActivationDto) {
        return new ResponseEntity<>(authService.resendActivation(resendActivationDto.getEmail()), HttpStatus.ACCEPTED);
    }
}
