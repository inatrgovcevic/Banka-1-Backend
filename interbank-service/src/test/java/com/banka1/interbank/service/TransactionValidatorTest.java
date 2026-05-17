package com.banka1.interbank.service;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertTrue;

import com.banka1.interbank.protocol.dto.Asset;
import com.banka1.interbank.protocol.dto.CurrencyCode;
import com.banka1.interbank.protocol.dto.ForeignBankId;
import com.banka1.interbank.protocol.dto.MonetaryAsset;
import com.banka1.interbank.protocol.dto.MonetaryValue;
import com.banka1.interbank.protocol.dto.NoVoteReason.Reason;
import com.banka1.interbank.protocol.dto.OptionDescription;
import com.banka1.interbank.protocol.dto.Posting;
import com.banka1.interbank.protocol.dto.StockDescription;
import com.banka1.interbank.protocol.dto.TxAccount;
import java.math.BigDecimal;
import java.time.OffsetDateTime;
import java.util.List;
import org.junit.jupiter.api.Test;

/**
 * PR_32 Phase 6 Task 6.1 unit testovi za {@link TransactionValidator}.
 *
 * <p>Pure-Java testovi bez Spring konteksta — validator je stateless.
 */
class TransactionValidatorTest {

    private final TransactionValidator v = new TransactionValidator();

    private Posting monas(String accountNum, String amount, CurrencyCode currency) {
        return new Posting(
                new TxAccount.Account(accountNum),
                new BigDecimal(amount),
                new Asset.Monas(new MonetaryAsset(currency)));
    }

    private Posting stock(int routingNumber, String personId, String amount, String ticker) {
        return new Posting(
                new TxAccount.Person(new ForeignBankId(routingNumber, personId)),
                new BigDecimal(amount),
                new Asset.Stock(new StockDescription(ticker)));
    }

    // ===== balanced check =====================================================

    @Test
    void balancedMonasIsValid() {
        Posting debitNa = monas("111000112345678901", "-1500", CurrencyCode.USD);
        Posting creditPartner = monas("222000112345678902", "1500", CurrencyCode.USD);
        assertTrue(v.checkBalanced(List.of(debitNa, creditPartner)).isEmpty());
    }

    @Test
    void unbalancedMonasFails() {
        Posting debitNa = monas("111000112345678901", "-1500", CurrencyCode.USD);
        Posting partial = monas("222000112345678902", "100", CurrencyCode.USD);
        var result = v.checkBalanced(List.of(debitNa, partial));
        assertTrue(result.isPresent());
        assertEquals(Reason.UNBALANCED_TX, result.get().reason());
    }

    @Test
    void differentCurrenciesNotMixed() {
        // USD postings sum=0 odvojeno, EUR sum=0 odvojeno → ukupno balansirano
        Posting usdD = monas("111000112345678901", "-100", CurrencyCode.USD);
        Posting usdC = monas("222000112345678902", "100", CurrencyCode.USD);
        Posting eurD = monas("111000112345678903", "-200", CurrencyCode.EUR);
        Posting eurC = monas("222000112345678904", "200", CurrencyCode.EUR);
        assertTrue(v.checkBalanced(List.of(usdD, usdC, eurD, eurC)).isEmpty());
    }

    @Test
    void unbalancedSecondCurrencyFails() {
        // USD balansiran, EUR ne — vraca UNBALANCED_TX
        Posting usdD = monas("111000112345678901", "-100", CurrencyCode.USD);
        Posting usdC = monas("222000112345678902", "100", CurrencyCode.USD);
        Posting eurD = monas("111000112345678903", "-200", CurrencyCode.EUR);
        Posting eurC = monas("222000112345678904", "150", CurrencyCode.EUR);
        var result = v.checkBalanced(List.of(usdD, usdC, eurD, eurC));
        assertTrue(result.isPresent());
        assertEquals(Reason.UNBALANCED_TX, result.get().reason());
    }

    @Test
    void stockBalanceChecked() {
        Posting sellerSide = stock(111, "C-5", "-10", "AAPL");
        Posting buyerSide = stock(222, "C-2", "10", "AAPL");
        assertTrue(v.checkBalanced(List.of(sellerSide, buyerSide)).isEmpty());
    }

    @Test
    void differentTickersGroupedSeparately() {
        // AAPL nije balansiran a TSLA jeste → ukupno UNBALANCED_TX
        Posting aaplD = stock(111, "C-5", "-10", "AAPL");
        Posting aaplCpartial = stock(222, "C-2", "5", "AAPL");
        Posting tslaD = stock(111, "C-5", "-3", "TSLA");
        Posting tslaC = stock(222, "C-2", "3", "TSLA");
        var result = v.checkBalanced(List.of(aaplD, aaplCpartial, tslaD, tslaC));
        assertTrue(result.isPresent());
        assertEquals(Reason.UNBALANCED_TX, result.get().reason());
    }

    @Test
    void optionPostingGroupedByNegotiationId() {
        // ista negotiation, balansirana
        OptionDescription opt = new OptionDescription(
                new ForeignBankId(222, "NEG-1"),
                new StockDescription("AAPL"),
                new MonetaryValue(CurrencyCode.USD, new BigDecimal("100")),
                OffsetDateTime.parse("2026-12-01T00:00:00Z"),
                10);
        Posting issuer = new Posting(
                new TxAccount.Option(new ForeignBankId(222, "NEG-1")),
                new BigDecimal("-1"),
                new Asset.Option(opt));
        Posting holder = new Posting(
                new TxAccount.Person(new ForeignBankId(111, "C-3")),
                new BigDecimal("1"),
                new Asset.Option(opt));
        assertTrue(v.checkBalanced(List.of(issuer, holder)).isEmpty());
    }

    // ===== ownership ==========================================================

    @Test
    void isOursMonasAccountByPrefix() {
        Posting p = monas("111000112345678901", "0", CurrencyCode.USD);
        assertTrue(v.isOursMonas(p, 111));
        assertFalse(v.isOursMonas(p, 222));
    }

    @Test
    void isOursMonasIgnoresPersonAccount() {
        // Person account je za stockove a ne MONAS; ova heuristika vraca false
        Posting p = stock(111, "C-5", "5", "AAPL");
        assertFalse(v.isOursMonas(p, 111));
    }

    @Test
    void isOursPersonByRoutingNumber() {
        Posting mine = stock(111, "C-5", "10", "AAPL");
        Posting theirs = stock(222, "C-9", "10", "AAPL");
        assertTrue(v.isOursPerson(mine, 111));
        assertFalse(v.isOursPerson(theirs, 111));
    }

    @Test
    void isOursOptionPseudoAccountByRouting() {
        OptionDescription optDesc = new OptionDescription(
                new ForeignBankId(222, "NEG-1"),
                new StockDescription("AAPL"),
                new MonetaryValue(CurrencyCode.USD, new BigDecimal("100")),
                OffsetDateTime.parse("2026-12-01T00:00:00Z"),
                10);
        Posting localOpt = new Posting(
                new TxAccount.Option(new ForeignBankId(111, "NEG-LOCAL")),
                new BigDecimal("-1"),
                new Asset.Option(optDesc));
        Posting remoteOpt = new Posting(
                new TxAccount.Option(new ForeignBankId(333, "NEG-REMOTE")),
                new BigDecimal("-1"),
                new Asset.Option(optDesc));
        assertTrue(v.isOursPerson(localOpt, 111));
        assertFalse(v.isOursPerson(remoteOpt, 111));
    }

    @Test
    void isOursPersonAccountByPrefix() {
        Posting p = monas("111000112345678901", "0", CurrencyCode.USD);
        assertTrue(v.isOursPerson(p, 111));
        assertFalse(v.isOursPerson(p, 999));
    }
}
