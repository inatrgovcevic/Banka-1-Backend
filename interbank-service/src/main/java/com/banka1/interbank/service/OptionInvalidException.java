package com.banka1.interbank.service;

/**
 * Option payload validacioni problem koji nije pokriven {@code NoVoteReason}
 * (npr. integritet checked tek u commit fazi, ili exercise dohvata istekao
 * ugovor).
 */
public class OptionInvalidException extends InterbankException {

    public OptionInvalidException(String message) {
        super(message);
    }
}
