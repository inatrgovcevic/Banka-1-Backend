package com.banka1.userservice.interbank.dto;

/**
 * PR_32 Phase 13: DTO koji vraca friendly display ime klijenta ili zaposlenog
 * za interbank resolve flow.
 *
 * <p>Koristi se iz {@code interbank-service.UserInternalClient} pri rendering-u
 * Trade ticket-a, OTC ugovora i SAGA event-a gde treba prikazati ime umesto
 * sirovog ID-a.
 *
 * @param firstName ime (Client.name ili Employee.ime); nikad null, prazno ako fali u izvoru
 * @param lastName prezime (Client.lastName ili Employee.prezime); nikad null
 * @param fullName "{@code firstName lastName}" trimovano, nikad null
 */
public record InterbankUserDisplayDto(String firstName, String lastName, String fullName) {
}
