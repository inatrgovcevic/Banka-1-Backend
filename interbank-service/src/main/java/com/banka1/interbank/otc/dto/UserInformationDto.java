package com.banka1.interbank.otc.dto;

/**
 * PR_32 Phase 10 Task 10.1: response payload za Tim 2 §3.7
 * (GET /interbank/user/{rn}/{id}). Vraca ljudski citljivo display ime
 * banke i korisnika za UI prikaz na strani partner banke.
 *
 * @param bankDisplayName ime nase banke (npr. "Banka 1")
 * @param displayName     ime + prezime korisnika (klijenta ili zaposlenog)
 */
public record UserInformationDto(String bankDisplayName, String displayName) {}
