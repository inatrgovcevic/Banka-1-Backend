package com.banka1.interbank.service;

import com.banka1.interbank.protocol.dto.Asset;
import com.banka1.interbank.protocol.dto.NoVoteReason;
import com.banka1.interbank.protocol.dto.NoVoteReason.Reason;
import com.banka1.interbank.protocol.dto.Posting;
import com.banka1.interbank.protocol.dto.TxAccount;
import java.math.BigDecimal;
import java.util.List;
import java.util.Map;
import java.util.Optional;
import java.util.stream.Collectors;
import org.springframework.stereotype.Component;

/**
 * PR_32 Phase 6 Task 6.1: helper koji obavlja preliminarne (cisto lokalne)
 * provere transakcije pre nego sto {@link TransactionExecutorService} udari u
 * REST pozive ka banking-core/trading-u.
 *
 * <p>Dve odgovornosti:
 * <ul>
 *   <li>{@link #checkBalanced(List)} — spec §2.8.3: za svaku Asset
 *       grupu (ista valuta / isti ticker / isti option negotiation ID) suma
 *       postings amount-a mora biti tacno 0. U suprotnom transakcija nije
 *       "balanced" i kandidat je za {@link Reason#UNBALANCED_TX}.</li>
 *   <li>{@link #isOursPerson(Posting, int)} — odredjuje da li posting pripada
 *       <em>nasoj</em> banci. Person ID i Option pseudo-account nose routing
 *       broj direktno; Account ima 18-digit broj racuna koji uvek pocinje sa
 *       3-cifrenim routing prefiksom banke vlasnika racuna.</li>
 * </ul>
 *
 * <p>Klasa namerno ne pristupa repository-jima ni REST klijentima — sve provere
 * su cisto pure funkcije nad payload-om. To olaksava unit testiranje
 * (Mockito nije potreban) i drzi {@link TransactionExecutorService} fokusiran
 * samo na orchestraciju.
 */
@Component
public class TransactionValidator {

    /**
     * Grupise postinge po Asset equivalence-u i proverava da li sum amount-a
     * unutar svake grupe iznosi tacno {@code 0}.
     *
     * @param postings lista postinga iz {@code InterbankTransactionPayload}
     * @return {@link Optional#empty()} ako su sve grupe balansirane; inace
     *         {@code Optional} sa {@link Reason#UNBALANCED_TX} razlogom (NE
     *         pripaja se konkretan posting jer nebalans pripada paru posting-a,
     *         ne pojedinacnom)
     */
    public Optional<NoVoteReason> checkBalanced(List<Posting> postings) {
        Map<String, BigDecimal> sumPerAssetKey = postings.stream().collect(
                Collectors.groupingBy(
                        this::assetKey,
                        Collectors.mapping(
                                Posting::amount,
                                Collectors.reducing(BigDecimal.ZERO, BigDecimal::add))));
        return sumPerAssetKey.entrySet().stream()
                .filter(e -> e.getValue().compareTo(BigDecimal.ZERO) != 0)
                .findFirst()
                .map(e -> NoVoteReason.of(Reason.UNBALANCED_TX));
    }

    /**
     * Vraca string kljuc koji identifikuje "isti" Asset za potrebe grupisanja:
     * MONAS razlikuje po valuti, STOCK po tickeru, OPTION po
     * {@code (routingNumber, id)} para negotiation ID-a.
     */
    String assetKey(Posting p) {
        return switch (p.asset()) {
            case Asset.Monas m -> "MONAS:" + m.asset().currency();
            case Asset.Stock s -> "STOCK:" + s.asset().ticker();
            case Asset.Option o -> "OPTION:"
                    + o.asset().negotiationId().routingNumber()
                    + ":"
                    + o.asset().negotiationId().id();
        };
    }

    /**
     * Convenience varijanta {@link #isOursPerson(Posting, int)} koja
     * razmatra <em>iskljucivo</em> {@link TxAccount.Account} postings, jer su
     * jedini relevantni za MONAS rezervacije (banking-core). Ako posting nije
     * Account-bazirani, vraca {@code false}.
     *
     * @param p posting
     * @param myRoutingNumber routing number nase banke iz
     *                        {@code interbank.my-routing-number}
     * @return true ako je posting Account sa prefiksom nase banke
     */
    public boolean isOursMonas(Posting p, int myRoutingNumber) {
        if (!(p.account() instanceof TxAccount.Account acc)) {
            return false;
        }
        return acc.num().startsWith(String.valueOf(myRoutingNumber));
    }

    /**
     * Generalna ownership provera za sve tri TxAccount varijante.
     *
     * <ul>
     *   <li>{@link TxAccount.Person} — proveravamo da {@code id.routingNumber}
     *       matchuje nasem.</li>
     *   <li>{@link TxAccount.Option} — pseudo-account za OTC opciju takodje
     *       nosi routing broj; isti princip.</li>
     *   <li>{@link TxAccount.Account} — 18-digit broj racuna pocinje sa 3-cifr.
     *       prefiksom banke vlasnika racuna.</li>
     * </ul>
     */
    public boolean isOursPerson(Posting p, int myRoutingNumber) {
        return switch (p.account()) {
            case TxAccount.Person pers -> pers.id().routingNumber() == myRoutingNumber;
            case TxAccount.Option opt -> opt.id().routingNumber() == myRoutingNumber;
            case TxAccount.Account a -> a.num().startsWith(String.valueOf(myRoutingNumber));
        };
    }
}
