package com.banka1.banking_service.transfer_service.client;

import com.banka1.banking_service.transfer_service.dto.client.ExchangeResponseDto;

import java.math.BigDecimal;

/**
 * Interfejs za komunikaciju sa servisom menjačnice (Exchange Service).
 */
public interface ExchangeClient {
    /**
     * Izračunava konverziju iz jedne valute u drugu uz primenu aktuelnog kursa i provizije.
     * @param fromCurrency izvorna valuta
     * @param toCurrency ciljna valuta
     * @param amount iznos koji se konvertuje
     * @return DTO sa detaljima konverzije, kursom i finalnim iznosom
     */
    ExchangeResponseDto calculateExchange(String fromCurrency, String toCurrency, BigDecimal amount);
}
