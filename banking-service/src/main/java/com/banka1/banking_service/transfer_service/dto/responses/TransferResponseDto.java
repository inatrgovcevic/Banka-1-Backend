package com.banka1.banking_service.transfer_service.dto.responses;

import lombok.Data;

import java.math.BigDecimal;
import java.time.Instant;

/**
 * Podaci o uspešno kreiranom transferu koji se vraćaju klijentu.
 */
@Data
public class TransferResponseDto {
    private String orderNumber; // Broj naloga
    private String fromAccountNumber; // Izvorni račun
    private String toAccountNumber; // Ciljni račun
    private BigDecimal initialAmount; // Originalni iznos
    private BigDecimal finalAmount; // Iznos nakon konverzije
    private BigDecimal exchangeRate; // Primenjeni kurs (ako postoji)
    private BigDecimal commission; // Provizija
    private Instant timestamp; // Vreme izvršenja
}
