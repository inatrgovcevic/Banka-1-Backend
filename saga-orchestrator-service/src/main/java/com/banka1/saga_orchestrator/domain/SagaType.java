package com.banka1.saga_orchestrator.domain;

/**
 * Tipovi SAGA-a koje orkestrator podržava.
 *
 * <p>PR_04 + PR_11 prosirenje:
 * <ul>
 *   <li>{@code OTC_EXERCISE} (Issue #220, 5 step-ova): rezervacija sredstava ka banking-core,
 *       provera/rezervacija hartija ka market-service, transfer sredstava, prenos vlasnistva,
 *       final consistency check.
 *   <li>{@code OTC_PREMIUM_TRANSFER} (PR_04 C4.4, 1 step): premium prenos kupac→prodavac
 *       posle ACCEPTED ponude.
 *   <li>{@code FUND_SUBSCRIBE} (PR_04 C4.9, 1 step): debit klijenta + credit fonda + create/update pozicije.
 *   <li>{@code FUND_REDEEM} (PR_04 C4.9, 1 step): fast path isplata kada fond ima dovoljno likvidnih.
 *   <li>{@code FUND_LIQUIDATION_FOR_REDEMPTION} (Issue #231, 2 step-a):
 *       likvidacija hartija fonda + transfer ka klijentu.
 * </ul>
 */
public enum SagaType {

    OTC_EXERCISE(5),
    OTC_PREMIUM_TRANSFER(1),
    FUND_SUBSCRIBE(1),
    FUND_REDEEM(1),
    FUND_LIQUIDATION_FOR_REDEMPTION(2);

    private final int totalSteps;

    SagaType(int totalSteps) {
        this.totalSteps = totalSteps;
    }

    public int getTotalSteps() {
        return totalSteps;
    }
}
