package com.banka1.interbank.service;

/**
 * Lookup po {@code (transactionIdRouting, transactionIdLocal)} 2PC kljucu nije
 * uspeo. Inbound controller mapira u 404.
 */
public class NoSuchTransactionException extends InterbankException {

    public NoSuchTransactionException(String message) {
        super(message);
    }
}
