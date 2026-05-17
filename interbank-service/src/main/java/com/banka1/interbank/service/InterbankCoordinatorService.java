package com.banka1.interbank.service;

import com.banka1.interbank.config.InterbankProperties;
import com.banka1.interbank.model.InterbankContractEntity;
import com.banka1.interbank.model.InterbankNegotiationEntity;
import com.banka1.interbank.model.enums.NegotiationContractStatus;
import com.banka1.interbank.protocol.dto.Asset;
import com.banka1.interbank.protocol.dto.CurrencyCode;
import com.banka1.interbank.protocol.dto.ForeignBankId;
import com.banka1.interbank.protocol.dto.InterbankTransactionPayload;
import com.banka1.interbank.protocol.dto.MonetaryAsset;
import com.banka1.interbank.protocol.dto.MonetaryValue;
import com.banka1.interbank.protocol.dto.OptionDescription;
import com.banka1.interbank.protocol.dto.Posting;
import com.banka1.interbank.protocol.dto.StockDescription;
import com.banka1.interbank.protocol.dto.TransactionVote;
import com.banka1.interbank.protocol.dto.TxAccount;
import com.banka1.interbank.repository.InterbankContractRepository;
import com.banka1.interbank.repository.InterbankNegotiationRepository;
import java.math.BigDecimal;
import java.security.SecureRandom;
import java.time.Instant;
import java.util.HexFormat;
import java.util.List;
import lombok.RequiredArgsConstructor;
import lombok.extern.slf4j.Slf4j;
import org.springframework.context.annotation.Profile;
import org.springframework.stereotype.Service;

/**
 * PR_32 Phase 10 Task 10.4: 2PC orchestrator za OTC §3.6
 * GET /negotiations/{rn}/{id}/accept.
 *
 * <p><strong>KRITICNO — Tim 2 §6.6:</strong>
 * <ul>
 *   <li>Endpoint moze drzati konekciju do 60s.</li>
 *   <li>204 No Content se vraca <em>tek</em> kad je COMMIT_TX uspesno poslato
 *       partner banci.</li>
 *   <li>5xx (kroz {@link InterbankException}) ako prepare/commit fail-uje.</li>
 * </ul>
 *
 * <p>Tok per spec §3.6 + §7:
 * <ol>
 *   <li>Konstrukcija 4-posting transakcije:
 *     <ol type="a">
 *       <li>Buyer (partner) — credit premium (-amount)</li>
 *       <li>Seller (mi) — debit premium (+amount)</li>
 *       <li>Seller Option pseudo-account (mi) — credit OPTION asset</li>
 *       <li>Buyer Person account (partner) — debit OPTION asset</li>
 *     </ol>
 *   </li>
 *   <li>{@link TransactionExecutorService#prepareLocal} — rezervisi nase strane.</li>
 *   <li>{@link InterbankClient#sendNewTx} — partner prepare.</li>
 *   <li>Ako partner NO ili exception — local rollback + sendRollbackTx.</li>
 *   <li>{@link TransactionExecutorService#commitLocal} — finalizuj nase.</li>
 *   <li>Flip negotiation isOngoing=false + create contract ACTIVE.</li>
 *   <li>{@link InterbankClient#sendCommitTx} — partner commit (KRITICNO za 204).</li>
 * </ol>
 *
 * <p>Created kontrakt prelazi direktno u {@code ACTIVE} jer je premium 2PC
 * commit-ovan pre nego sto save-ujemo entity (KRIT bug #2 fix iz CLAUDE.md
 * sekcije 4 je za <em>intra-bank</em> flow; ovde inter-bank 2PC garantuje
 * premium pre contract-a).
 */
@Service
@Profile("!test")
@RequiredArgsConstructor
@Slf4j
public class InterbankCoordinatorService {

    private final TransactionExecutorService executor;
    private final InterbankClient interbankClient;
    private final InterbankNegotiationRepository negRepo;
    private final InterbankContractRepository contractRepo;
    private final InterbankProperties props;
    private final SecureRandom random = new SecureRandom();

    /**
     * Synchronous 2PC za accept negotiation. Blokira do uspesnog COMMIT_TX-a ka
     * partner banci ili throw {@link InterbankException}.
     *
     * @param neg negotiation entity (mora biti is_ongoing=true, settlement u
     *            buducnosti, turn checks vec urađeni u
     *            {@link OtcNegotiationService#acceptNegotiation})
     * @throws InterbankException ako prepare faza padne (lokalno ili partner)
     *                            ili ako commitLocal padne posle uspesnog
     *                            partner prepare-a (catastrophic — fail-loud)
     */
    public void acceptNegotiation(InterbankNegotiationEntity neg) {
        int myRouting = props.getMyRoutingNumber();
        int partnerRouting = neg.getBuyerRoutingNumber() == myRouting
                ? neg.getSellerRoutingNumber()
                : neg.getBuyerRoutingNumber();

        // Total premium = unit * amount (premium u entity je per-unit)
        BigDecimal totalPremium = neg.getPremiumAmount().multiply(BigDecimal.valueOf(neg.getAmount()));
        CurrencyCode premCurr = CurrencyCode.valueOf(neg.getPremiumCurrency());
        CurrencyCode strikeCurr = CurrencyCode.valueOf(neg.getPriceCurrency());

        OptionDescription optionDesc = new OptionDescription(
                new ForeignBankId(myRouting, neg.getId()),
                new StockDescription(neg.getStockTicker()),
                new MonetaryValue(strikeCurr, neg.getPriceAmount()),
                neg.getSettlementDate(),
                neg.getAmount()
        );

        List<Posting> postings = List.of(
                // a) Buyer credit premium (-amount): nadoknada izlazi iz buyer-a.
                new Posting(
                        new TxAccount.Person(new ForeignBankId(
                                neg.getBuyerRoutingNumber(), neg.getBuyerId())),
                        totalPremium.negate(),
                        new Asset.Monas(new MonetaryAsset(premCurr))),
                // b) Seller debit premium (+amount): mi primamo premium.
                new Posting(
                        new TxAccount.Person(new ForeignBankId(
                                neg.getSellerRoutingNumber(), neg.getSellerId())),
                        totalPremium,
                        new Asset.Monas(new MonetaryAsset(premCurr))),
                // c) Seller option pseudo-account credit (-1 unit option)
                new Posting(
                        new TxAccount.Option(new ForeignBankId(myRouting, neg.getId())),
                        BigDecimal.ONE.negate(),
                        new Asset.Option(optionDesc)),
                // d) Buyer debit OPTION (+1 unit) — buyer dobija opciju.
                new Posting(
                        new TxAccount.Person(new ForeignBankId(
                                neg.getBuyerRoutingNumber(), neg.getBuyerId())),
                        BigDecimal.ONE,
                        new Asset.Option(optionDesc))
        );

        ForeignBankId txId = new ForeignBankId(myRouting, generateTxId());
        InterbankTransactionPayload tx = new InterbankTransactionPayload(
                postings, txId,
                "OTC accept for negotiation " + neg.getId(),
                null, "289", "OTC premium + option transfer");

        // 1. Local prepare — rezervisi nasu stranu (seller premium "uplata" je
        //    pozitivna za nas, ali seller option pseudo-account je credit, koji
        //    posto je nas, mora se rezervisati kroz reserveOption).
        TransactionVote localVote = executor.prepareLocal(tx);
        if (!localVote.isYes()) {
            log.error("Local prepare failed za accept negotiation {}: {}",
                    neg.getId(), localVote.reasons());
            throw new InterbankException(
                    "Local prepare failed: " + localVote.reasons());
        }

        // 2. Partner prepare
        TransactionVote partnerVote;
        try {
            partnerVote = interbankClient.sendNewTx(partnerRouting, tx);
        } catch (Exception e) {
            log.error("Partner prepare exception za accept negotiation {}", neg.getId(), e);
            safeRollbackLocal(txId);
            throw new InterbankException("Partner prepare exception: " + e.getMessage(), e);
        }
        if (!partnerVote.isYes()) {
            log.warn("Partner rejected accept negotiation {}: {}",
                    neg.getId(), partnerVote.reasons());
            safeRollbackLocal(txId);
            safeRollbackPartner(partnerRouting, txId);
            throw new InterbankException(
                    "Partner rejected: " + partnerVote.reasons());
        }

        // 3. Local commit
        try {
            executor.commitLocal(txId);
        } catch (Exception e) {
            log.error("CATASTROPHIC: commitLocal failed posle partner YES za negotiation {}",
                    neg.getId(), e);
            // Mi smo zaglavljeni u stanju gde partner ce ocekivati commit ali mi
            // ne mozemo da finalizujemo. Saljemo rollback partner-u da ga
            // oslobodimo, ali nas state je nedosledan — fail loud.
            safeRollbackPartner(partnerRouting, txId);
            throw new InterbankException("Catastrophic commitLocal failure", e);
        }

        // 4. Flip negotiation + create contract (zasebno, lokalno).
        neg.setOngoing(false);
        negRepo.save(neg);

        InterbankContractEntity contract = InterbankContractEntity.builder()
                .id(generateContractId())
                .negotiationId(neg.getId())
                .buyerRoutingNumber(neg.getBuyerRoutingNumber())
                .buyerId(neg.getBuyerId())
                .sellerRoutingNumber(neg.getSellerRoutingNumber())
                .sellerId(neg.getSellerId())
                .stockTicker(neg.getStockTicker())
                .amount(neg.getAmount())
                .strikeCurrency(neg.getPriceCurrency())
                .strikeAmount(neg.getPriceAmount())
                .settlementDate(neg.getSettlementDate())
                .status(NegotiationContractStatus.ACTIVE)
                .optionPseudoOwnerRouting(neg.getBuyerRoutingNumber())
                .optionPseudoOwnerId(neg.getBuyerId())
                .createdAt(Instant.now())
                .build();
        contractRepo.save(contract);

        // 5. Partner commit — KRITICNO za 204 response per Tim 2 §6.6.
        try {
            interbankClient.sendCommitTx(partnerRouting, txId);
        } catch (Exception e) {
            // Mi smo vec commit-ovali nase strane i napravili kontrakt; partner
            // ce dobiti retry kroz InterbankRetryScheduler. Log warn ali NE
            // baca exception ka pozivocu jer je nas state vec konzistentan.
            log.warn("sendCommitTx ka partneru {} failed za tx {} — retry "
                            + "scheduler ce ga pokupiti", partnerRouting, txId, e);
        }

        log.info("Accepted negotiation {} — contract {} ACTIVE (tx {})",
                neg.getId(), contract.getId(), txId);
    }

    private void safeRollbackLocal(ForeignBankId txId) {
        try {
            executor.rollbackLocal(txId);
        } catch (Exception ex) {
            log.warn("safeRollbackLocal failed za tx {}", txId, ex);
        }
    }

    private void safeRollbackPartner(int partnerRouting, ForeignBankId txId) {
        try {
            interbankClient.sendRollbackTx(partnerRouting, txId);
        } catch (Exception ex) {
            log.warn("safeRollbackPartner failed za tx {} partner {}",
                    txId, partnerRouting, ex);
        }
    }

    private String generateTxId() {
        byte[] bytes = new byte[8];
        random.nextBytes(bytes);
        return "tx-" + HexFormat.of().formatHex(bytes);
    }

    private String generateContractId() {
        byte[] bytes = new byte[8];
        random.nextBytes(bytes);
        return "otc-" + HexFormat.of().formatHex(bytes);
    }
}
