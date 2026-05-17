package com.banka1.interbank.service;

/**
 * Pokusaj commit-a (ili rollback-a) iz terminalnog stanja: transakcija je vec
 * COMMITTED/ROLLED_BACK/FAILED. Idempotency u {@link TransactionExecutorService}
 * tretira ponovljeni isti operator kao no-op; ova ekscepcija se baca samo kada
 * je trazena tranzicija nedozvoljena (npr. commit nakon rollback-a).
 */
public class AlreadyCommittedException extends InterbankException {

    public AlreadyCommittedException(String message) {
        super(message);
    }
}
