package com.banka1.account_service.domain;

/**
 * PR_29: Konstante za sistemske ID-eve vlasnika racuna.
 *
 * <p>Spec (Celina 2: "Naša Banka = Firma", Celina 3: "Naša država = Firma"):
 * banka i drzava se modeluju kao firme sa sopstvenim racunima koji se koriste
 * za:
 * <ul>
 *   <li>Banka: provizije, menjacnica internal flow, bank-to-exchange margin transfer.</li>
 *   <li>Drzava: porez na kapitalnu dobit, settlement OTC opcionog ugovora.</li>
 * </ul>
 *
 * <p>U bazi se ovi entiteti razlikuju od regularnih klijenata po negativnim
 * vrednostima u {@code accounts.vlasnik} koloni. Pre PR_29, hardkodi {@code -1} i
 * {@code -2} bili su prosuti kroz JPQL query-je u {@code AccountRepository} i
 * service kodu, sto je mestilo i otezavalo refaktor (npr. ako se dodaju novi
 * tipovi sistemskih entiteta).
 *
 * <p>Liquibase changelog-ovi (npr. {@code 005-seed-state-company-and-account.sql})
 * referenciraju iste vrednosti — ako se ovde menjaju, mora i tamo.
 */
public final class SystemAccountIds {

    /** ID vlasnika za sve racune koje poseduje banka (Banka1 kao Firma). */
    public static final long BANK = -1L;

    /** ID vlasnika za sve racune koje poseduje drzava (Naša država kao Firma). */
    public static final long STATE = -2L;

    /**
     * ID vlasnika za sve racune koje poseduje (interni) exchange entitet,
     * koriscen u margin transferu izmedu banke i berze.
     */
    public static final long EXCHANGE = -3L;

    private SystemAccountIds() {
        // utility class
    }
}
