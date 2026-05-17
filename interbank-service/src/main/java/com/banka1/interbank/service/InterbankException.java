package com.banka1.interbank.service;

/**
 * PR_32 Phase 6 Task 6.2: bazna runtime-exception klasa za interbank service
 * sloj. Inbound controller (Phase 7) hvata ovu klasu (i podklase) i mapira u
 * odgovarajuce HTTP odgovore.
 *
 * <p>Cetiri podklase u istom paketu:
 * <ul>
 *   <li>{@link NoSuchTransactionException} — 404 lookup po 2PC kljucu</li>
 *   <li>{@link AlreadyCommittedException} — pokusaj commit-a/rollback-a iz
 *       terminalnog stanja (COMMITTED/ROLLED_BACK/FAILED)</li>
 *   <li>{@link NegotiationNotFoundException} — option negotiation ne postoji u
 *       lokalnom mirror-u</li>
 *   <li>{@link OptionInvalidException} — option payload validacioni problem
 *       koji nije pokriven {@code NoVoteReason} (npr. razmotaj u commit-time-u)</li>
 * </ul>
 */
public class InterbankException extends RuntimeException {

    public InterbankException(String message) {
        super(message);
    }

    public InterbankException(String message, Throwable cause) {
        super(message, cause);
    }
}
