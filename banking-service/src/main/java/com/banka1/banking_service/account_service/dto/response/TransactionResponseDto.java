package com.banka1.banking_service.account_service.dto.response;

/**
 * DTO za odgovor sa informacijama o izvršenoj transakciji.
 * <p>
 * Koristi se za vraćanje detaljnih informacija o transakciji klijentima nakon
 * što je transakcija izvršena. Planirana je za proširenje sa detaljima kao što su
 * ID transakcije, vremenski pečat, iznos, valuta, status i eventualne greške.
 * <p>
 * Napomena: Trenutna implementacija je prazna i čeka dodelu polja.
 * Očekuju se polja:
 * <ul>
 *   <li>{@code transactionId} - jedinstveni identifikator transakcije</li>
 *   <li>{@code status} - status transakcije (PENDING, COMPLETED, FAILED)</li>
 *   <li>{@code timestamp} - vremenske oznake izvršavanja</li>
 *   <li>{@code amount} - iznos transakcije</li>
 *   <li>{@code currency} - valuta transakcije</li>
 *   <li>{@code description} - opis ili napomena o transakciji</li>
 * </ul>
 */
public class TransactionResponseDto {
}
