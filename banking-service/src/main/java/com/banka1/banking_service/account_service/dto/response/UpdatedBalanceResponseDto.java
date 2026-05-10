package com.banka1.banking_service.account_service.dto.response;

import lombok.AllArgsConstructor;
import lombok.Getter;
import lombok.Setter;

import java.math.BigDecimal;

/**
 * DTO za odgovor sa ažuriranim stanjem računa nakon izvršene transakcije.
 * <p>
 * Koristi se za vraćanje konačnih stanja računa pošiljaoca i primaoca nakon
 * što je transakcija uspešno izvršena. Omogućava klijentima da vide promene
 * u realnom vremenu bez dodatnog upita ka serveru.
 */
@Getter
@Setter
@AllArgsConstructor
public class UpdatedBalanceResponseDto {
    /**
     * Novo stanje računa pošiljaoca nakon transakcije.
     * Ovo je konačno stanje nakon odbitka transferovane sume i komisije.
     */
    private BigDecimal senderBalance;

    /**
     * Novo stanje računa primaoca nakon transakcije.
     * Ovo je konačno stanje nakon dodavanja transferovane sume (konvertovane ako je potrebno).
     */
    private BigDecimal receiverBalance;
}
