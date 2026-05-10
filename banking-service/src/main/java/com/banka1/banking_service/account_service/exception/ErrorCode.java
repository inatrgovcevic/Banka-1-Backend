package com.banka1.banking_service.account_service.exception;

import lombok.Getter;
import org.springframework.http.HttpStatus;

/**
 * Enum koji centralizuje sve poslovne greške aplikacije specifične za account-service.
 * <p>
 * Svaka konstanta sadrži:
 * <ul>
 *   <li>HTTP status - status kod koji će biti vraćen klijentu</li>
 *   <li>Maćinski-čitljivi kod - stabilan identifikator greške (npr. ERR_ACCOUNT_001)</li>
 *   <li>Naslov - kratka, ljudski-čitljiva poruka greške</li>
 * </ul>
 * <p>
 * Greške se vraćaju klijentu kroz {@link BusinessException} i obrađuju se
 * u {@code GlobalExceptionHandler}-u koji generiše standardizovani {@code ErrorResponseDto}.
 * <p>
 * Preporučuje se koristiti ove kodove u logima i monitor-anju za lakšu dijagnostiku.
 */
@Getter
public enum ErrorCode {

    // ── (ERR_ACCOUNT_xxx) ─────────────────────────────────────

    /**
     * Greška kada račun nema dovoljno sredstava za izvršavanje transakcije.
     * <p>
     * Obično znači da je raspoloživo stanje manje od tražene sume plus komisija.
     */
    INSUFFICIENT_FUNDS(HttpStatus.UNPROCESSABLE_CONTENT, "ERR_ACCOUNT_001", "Nema dovoljno novca na računu"),

    /**
     * Greška kada bi transakcija prekoračila dnevni limit trošenja.
     * <p>
     * Korisnik je dostigao/prekoračio svoju dnevnu graničnu sumu.
     */
    DAILY_LIMIT_EXCEEDED(HttpStatus.UNPROCESSABLE_CONTENT, "ERR_ACCOUNT_002", "Pređen dnevni limit"),

    /**
     * Greška kada bi transakcija prekoračila mesečni limit trošenja.
     * <p>
     * Korisnik je dostigao/prekoračio svoju mesečnu graničnu sumu.
     */
    MONTHLY_LIMIT_EXCEEDED(HttpStatus.UNPROCESSABLE_CONTENT, "ERR_ACCOUNT_003", "Pređen mesečni limit"),

    /**
     * Greška kada verifikacija (npr. kod iz mobilne aplikacije) nije uspela.
     * <p>
     * Obično znači da je kod istekao, neispravna ili pokušaj nije mogao biti validiran.
     */
    VERIFICATION_FAILED(HttpStatus.FORBIDDEN, "ERR_ACCOUNT_004", "Neuspešna verifikacija");

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
