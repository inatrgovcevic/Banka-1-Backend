package com.banka1.tradingservice.otc.domain;

/**
 * Status zivotnog ciklusa OTC opcionog ugovora.
 *
 * <p>Flow (PR_32 Phase 12 KRIT #2 fix):
 * <pre>
 *   PENDING_PREMIUM --(saga premium transfer success)--> ACTIVE --(buyer exercise)--> EXERCISED
 *                  \                                          \--(settlementDate prosao)-> EXPIRED
 *                   \--(saga premium transfer failed)--> CANCELED
 * </pre>
 *
 * <ul>
 *   <li>{@code PENDING_PREMIUM} — ugovor je upravo kreiran (seller accept), ali
 *       SAGA OTC_PREMIUM_TRANSFER jos nije zavrsila. Akcije <b>jesu</b>
 *       rezervisane (reservedQuantity), premija jos nije prebacena.</li>
 *   <li>{@code ACTIVE} — premija uspesno prebacena; kupac moze "Iskoristi"
 *       opciju do {@code settlementDate}.</li>
 *   <li>{@code EXERCISED} — kupac iskoristio opciju (SAGA OTC_EXERCISE
 *       completed). Akcije transferovane na kupca, novac na prodavca.</li>
 *   <li>{@code EXPIRED} — settlementDate prosao bez exercise-a; rezervisane
 *       akcije prodavca se oslobadjaju.</li>
 *   <li>{@code CANCELED} — SAGA premium transfer failed (kupac nema dovoljno
 *       sredstava). Ugovor se gasi, rezervisane akcije se oslobadjaju.</li>
 * </ul>
 */
public enum OptionContractStatus {
    PENDING_PREMIUM,
    ACTIVE,
    EXERCISED,
    EXPIRED,
    CANCELED
}
