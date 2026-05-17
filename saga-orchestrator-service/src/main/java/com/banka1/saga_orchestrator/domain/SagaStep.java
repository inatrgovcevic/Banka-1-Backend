package com.banka1.saga_orchestrator.domain;

/**
 * Jedan korak SAGA tokova. Implementacija živi u zasebnoj klasi za svaki
 * (sagaType, stepIndex) par. Po DoD-u Issue #214: {@code execute} šalje komandu
 * preko RabbitMQ, {@code compensate} undo-uje promenu kad neki naredni korak
 * vrati failure.
 */
public interface SagaStep {

    /**
     * Tip SAGE u kojoj korak učestvuje.
     */
    SagaType sagaType();

    /**
     * Redni broj koraka unutar SAGE (počinje od 0).
     */
    int stepIndex();

    /**
     * Slanje komande ka odgovornom servisu (banking/order/otc/fund) preko
     * RabbitMQ. Ne čeka odgovor sinkrono - odgovor stiže kao event na
     * {@code saga.events} queue.
     *
     * @param instance perzistentna SAGA instance koja drži payload i state
     */
    void execute(SagaInstance instance);

    /**
     * Kompenzacija prethodno uspešno izvršenog koraka. Pokreće se u obrnutom
     * redosledu kad neki kasniji korak failuje. Mora biti idempotentno.
     *
     * @param instance perzistentna SAGA instance
     */
    void compensate(SagaInstance instance);
}
