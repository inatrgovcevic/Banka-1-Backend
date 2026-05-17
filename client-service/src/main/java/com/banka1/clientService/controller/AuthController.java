package com.banka1.clientService.controller;

import com.banka1.clientService.dto.requests.ActivateDto;
import com.banka1.clientService.dto.requests.ForgotPasswordDto;
import com.banka1.clientService.dto.requests.LoginRequestDto;
import com.banka1.clientService.dto.responses.CheckActivateDto;
import com.banka1.clientService.dto.responses.LoginResponseDto;
import com.banka1.clientService.service.AuthService;
import jakarta.validation.Valid;
import lombok.AllArgsConstructor;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.*;

/**
 * REST kontroler koji izlaze endpoint-e za autentifikaciju i upravljanje nalogom klijenata.
 * Svi endpoint-i su dostupni pod baznom putanjom {@code /auth}.
 */
@RestController("clientAuthController")
@RequestMapping("/clients/auth")
@AllArgsConstructor
public class AuthController {

    /** Servis koji sadrzi poslovnu logiku autentifikacije i upravljanja nalogom. */
    private final AuthService authService;

    /**
     * Autentifikuje klijenta i vraca JWT pristupni token.
     *
     * @param dto kredencijali za prijavu
     * @return odgovor sa JWT tokenom
     */
    @PostMapping("/login")
    public ResponseEntity<LoginResponseDto> login(@RequestBody @Valid LoginRequestDto dto) {
        return ResponseEntity.ok(authService.login(dto));
    }

    /**
     * Proverava validnost aktivacionog tokena i vraca ID potvrde.
     *
     * @param token nehesirani URL-safe token primljen iz aktivacionog linka
     * @return DTO sa identifikatorom confirmation tokena
     */
    @GetMapping("/check-activate")
    public ResponseEntity<CheckActivateDto> checkActivate(@RequestParam String token) {
        return ResponseEntity.ok(new CheckActivateDto(authService.check(token)));
    }

    /**
     * Aktivira nalog klijenta i postavlja inicijalnu lozinku.
     *
     * @param dto podaci sa ID-em potvrde, tokenom i novom lozinkom
     * @return poruka o uspesnoj aktivaciji
     */
    @PostMapping("/activate")
    public ResponseEntity<String> activate(@RequestBody @Valid ActivateDto dto) {
        return ResponseEntity.ok(authService.editPassword(dto, true));
    }

    /**
     * Resetuje lozinku vec aktiviranog klijenta.
     *
     * @param dto podaci sa ID-em potvrde, tokenom i novom lozinkom
     * @return poruka o uspesnoj promeni lozinke
     */
    @PostMapping("/reset-password")
    public ResponseEntity<String> resetPassword(@RequestBody @Valid ActivateDto dto) {
        return ResponseEntity.ok(authService.editPassword(dto, false));
    }

    /**
     * Pokrecce slanje linka za reset lozinke na email klijenta.
     *
     * @param dto DTO sa email adresom klijenta
     * @return poruka o rezultatu operacije
     */
    @PostMapping("/forgot-password")
    public ResponseEntity<String> forgotPassword(@RequestBody @Valid ForgotPasswordDto dto) {
        return ResponseEntity.ok(authService.forgotPassword(dto));
    }

    /**
     * Ponovo salje aktivacioni mejl za neaktiviran nalog.
     *
     * @param dto DTO sa email adresom klijenta
     * @return poruka o rezultatu operacije
     */
    @PostMapping("/resend-activation")
    public ResponseEntity<String> resendActivation(@RequestBody @Valid ForgotPasswordDto dto) {
        return ResponseEntity.ok(authService.resendActivation(dto.getEmail()));
    }
}
