package com.banka1.saga_orchestrator.domain;

/**
 * Lifecycle SAGA instance-a. Tranzicije:
 * <pre>
 *   STARTED -> IN_PROGRESS -> COMPLETED
 *                          \-> COMPENSATING -> FAILED
 * </pre>
 *
 * STARTED je trenutak kad je instance perzistirana ali nijedan korak nije
 * poslat. IN_PROGRESS dok se koraci izvršavaju i odgovori akumuliraju.
 * COMPENSATING kada neki korak vrati failure event i orchestrator izvršava
 * compensation u obrnutom redosledu. Terminalni stati su COMPLETED i FAILED.
 */
public enum SagaState {
    STARTED,
    IN_PROGRESS,
    COMPENSATING,
    COMPLETED,
    FAILED
}
