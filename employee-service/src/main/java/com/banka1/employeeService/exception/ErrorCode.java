package com.banka1.employeeService.exception;

import org.springframework.http.HttpStatus;
import lombok.Getter;

/**
 * Enum koji centralizuje sve poslovne greske aplikacije.
 * Svaka konstanta nosi HTTP status, masinsko-citljivi kod i kratak naslov koji se
 * vracaju klijentu putem {@link BusinessException} i {@code GlobalExceptionHandler}-a.
 */
@Getter
public enum ErrorCode {

    // ── Korisnicke greske (ERR_USER_xxx) ─────────────────────────────────────

    /** Zaposleni sa trazenim identifikatorom nije pronadjen u bazi. */
    USER_NOT_FOUND(HttpStatus.NOT_FOUND, "ERR_USER_001", "Zaposleni nije pronađen"),

    /** Pokusaj kreiranja zaposlenog sa email adresom koja vec postoji. */
    EMAIL_ALREADY_EXISTS(HttpStatus.CONFLICT, "ERR_USER_002", "Email adresa je već u upotrebi"),

    /** Pokusaj kreiranja zaposlenog sa korisnickim imenom koje je vec zauzeto. */
    USERNAME_ALREADY_EXISTS(HttpStatus.CONFLICT, "ERR_USER_003", "Korisničko ime je već zauzeto"),

    /** Korisnik nema dovoljno jaku ulogu za izvrsavanje zatrazene operacije. */
    NOT_STRONG_ROLE(HttpStatus.BAD_REQUEST, "ERR_USER_004", "Nemas dovoljnu rolu"),

    /** Pokusaj kreiranja zaposlenog koji nije punoletan (mladji od 18 godina). */
    USER_TOO_YOUNG(HttpStatus.BAD_REQUEST, "ERR_USER_005", "Korisnik mora biti punoletan"),

    // ── Autentifikacione greske (ERR_AUTH_xxx) ────────────────────────────────

    /** Neispravni kredencijali pri prijavi (pogresan email ili lozinka). */
    INVALID_CREDENTIALS(HttpStatus.UNAUTHORIZED, "ERR_AUTH_001", "Neispravni kredencijali"),

    /** Nevazeci, istekli ili nepostojeci token. */
    INVALID_TOKEN(HttpStatus.BAD_REQUEST, "ERR_AUTH_002", "Neispravan token"),

    /** Token je istekao (Spec Celina 1, Sc 9: "Link za aktivaciju je istekao"). */
    TOKEN_EXPIRED(HttpStatus.BAD_REQUEST, "ERR_AUTH_006", "Link za aktivaciju je istekao"),

    /** Nalog privremeno zakljucan zbog previse neuspesnih pokusaja (Spec Celina 1, Sc 5). */
    ACCOUNT_LOCKED(HttpStatus.FORBIDDEN, "ERR_AUTH_007", "Nalog je privremeno zaključan zbog previše neuspešnih pokušaja"),

    /** Korisnik postoji, ali mu nalog nije aktivan. */
    USER_INACTIVE(HttpStatus.FORBIDDEN, "ERR_AUTH_003", "Korisnik nije aktivan"),

    /** Korisnik je soft-obrisan i ne moze da se autentifikuje. */
    USER_DELETED(HttpStatus.NOT_FOUND, "ERR_AUTH_004", "Korisnik je obrisan"),

    /** Interna greska pri generisanju refresh ili confirmation tokena (kolizija posle vise pokusaja). */
    TOKEN_GENERATION_FAILED(HttpStatus.INTERNAL_SERVER_ERROR, "ERR_AUTH_005", "Greška pri generisanju tokena");

    /** HTTP status koji se vraca klijentu kada se baci ova greska. */
    private final HttpStatus httpStatus;

    /** Stabilan masinsko-citljivi identifikator greske (npr. {@code "ERR_USER_001"}). */
    private final String code;

    /** Kratak ljudski citljivi naslov greske. */
    private final String title;

    /**
     * Kreira konstantu greske sa zadatim HTTP statusom, kodom i naslovom.
     *
     * @param httpStatus HTTP status koji se vraca klijentu
     * @param code stabilan identifikator greske
     * @param title kratak naslov greske
     */
    ErrorCode(HttpStatus httpStatus, String code, String title) {
        this.httpStatus = httpStatus;
        this.code = code;
        this.title = title;
    }
}
