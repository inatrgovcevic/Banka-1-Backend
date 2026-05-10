package com.banka1.banking_service.transfer_service.exception;

import lombok.Getter;
import org.springframework.http.HttpStatus;

/**
 * Enum koji centralizuje definicije svih poslovnih grešaka u sistemu.
 * Svaka konstanta definiše HTTP status, jedinstveni kod (npr. ERR_TRF_001) i kratak naslov greške.
 */
@Getter
public enum ErrorCode {

    // ── Transfer greske (ERR_TRF_xxx) ─────────────────────────────────────

    TRANSFER_NOT_FOUND(HttpStatus.NOT_FOUND, "ERR_TRF_001", "Transfer nije pronađen"), // Resurs ne postoji

    ACCOUNT_OWNERSHIP_MISMATCH(HttpStatus.FORBIDDEN, "ERR_TRF_002", "Računi ne pripadaju istom klijentu"), // Sigurnosna validacija vlasništva

    SAME_ACCOUNT_TRANSFER(HttpStatus.BAD_REQUEST, "ERR_TRF_003", "Izvorni i ciljni račun ne mogu biti isti"), // Logička validacija

    INVALID_VERIFICATION(HttpStatus.BAD_REQUEST, "ERR_TRF_004", "Neispravan ili istekao verifikacioni kod"), // 2FA validacija

    ACCOUNT_NOT_FOUND(HttpStatus.NOT_FOUND, "ERR_TRF_005", "Jedan od unetih računa ne postoji"), // Nepostojeći računi u sistemu

    INSUFFICIENT_FUNDS(HttpStatus.BAD_REQUEST, "ERR_TRF_006", "Nedovoljno raspoloživih sredstava na računu"), // Nedovoljno sredstava (Account service validacija)

    TRANSFER_ALREADY_PROCESSED(HttpStatus.CONFLICT, "ERR_TRF_007", "Transfer je već procesuiran"); // Idempotencija

    private final HttpStatus httpStatus;
    private final String code;
    private final String title;

    ErrorCode(HttpStatus httpStatus, String code, String title) {
        this.httpStatus = httpStatus;
        this.code = code;
        this.title = title;
    }
}