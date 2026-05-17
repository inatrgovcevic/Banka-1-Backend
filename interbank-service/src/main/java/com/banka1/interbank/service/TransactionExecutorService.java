package com.banka1.interbank.service;

import com.banka1.interbank.client.BankingCoreInternalClient;
import com.banka1.interbank.client.TradingInternalClient;
import com.banka1.interbank.config.InterbankProperties;
import com.banka1.interbank.model.InterbankTransactionEntity;
import com.banka1.interbank.model.enums.TxStatus;
import com.banka1.interbank.protocol.dto.Asset;
import com.banka1.interbank.protocol.dto.ForeignBankId;
import com.banka1.interbank.protocol.dto.InterbankTransactionPayload;
import com.banka1.interbank.protocol.dto.NoVoteReason;
import com.banka1.interbank.protocol.dto.NoVoteReason.Reason;
import com.banka1.interbank.protocol.dto.Posting;
import com.banka1.interbank.protocol.dto.TransactionVote;
import com.banka1.interbank.protocol.dto.TxAccount;
import com.banka1.interbank.repository.InterbankContractRepository;
import com.banka1.interbank.repository.InterbankNegotiationRepository;
import com.banka1.interbank.repository.InterbankTransactionRepository;
import com.fasterxml.jackson.core.type.TypeReference;
import com.fasterxml.jackson.databind.ObjectMapper;
import java.math.BigDecimal;
import java.time.Instant;
import java.time.OffsetDateTime;
import java.util.ArrayList;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.UUID;
import lombok.RequiredArgsConstructor;
import lombok.extern.slf4j.Slf4j;
import org.springframework.context.annotation.Profile;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;

/**
 * PR_32 Phase 6 Task 6.2-6.6: koordinator interbank 2PC izvodjenja.
 *
 * <p>Implementira tri javne operacije iz spec §7:
 * <ul>
 *   <li>{@link #prepareLocal(InterbankTransactionPayload)} — NEW_TX faza:
 *       validacija + saga-style rezervacija + persist PREPARED entity.</li>
 *   <li>{@link #commitLocal(ForeignBankId)} — COMMIT_TX faza: finalizacija
 *       svih rezervacija.</li>
 *   <li>{@link #rollbackLocal(ForeignBankId)} — ROLLBACK_TX faza:
 *       kompenzacija rezervacija + flip status u ROLLED_BACK.</li>
 * </ul>
 *
 * <p><strong>KRITICAN INVARIANTNI PATTERN:</strong> Spring {@code @Transactional}
 * annotation pokriva <em>samo</em> lokalnu interbank-service DB. Outbound REST
 * pozivi ka banking-core/trading-u nisu pokriveni Spring rollback-om, jer imaju
 * svoju nezavisnu DB transakciju u remote servisu. Zato koristimo
 * <em>saga-style</em> kompenzaciju (try-catch + eksplicitni release) kad
 * rezervacija fail-uje sredinom batch-a. Persist PREPARED entity-ja se dogadja
 * <em>posle</em> uspesno-zavrsene rezervacije, u zasebnoj {@code @Transactional}
 * metodi {@link #persistPrepared}.
 */
@Service
@Profile("!test")
@RequiredArgsConstructor
@Slf4j
public class TransactionExecutorService {

    private static final String REF_TYPE = "type";
    private static final String REF_ID = "id";

    private final TransactionValidator validator;
    private final BankingCoreInternalClient bankingCore;
    private final TradingInternalClient trading;
    private final InterbankProperties props;
    private final InterbankTransactionRepository txRepo;
    private final InterbankNegotiationRepository negRepo;
    private final InterbankContractRepository contractRepo;
    private final ObjectMapper mapper;

    /**
     * NEW_TX prepare per spec §7.1.
     *
     * @return YES ako sva validacija + rezervacija prosla, inace NO sa listom
     *         razloga
     */
    public TransactionVote prepareLocal(InterbankTransactionPayload tx) {
        // 1. Balanced check (cisto lokalno)
        var balanceErr = validator.checkBalanced(tx.postings());
        if (balanceErr.isPresent()) {
            return TransactionVote.no(List.of(balanceErr.get()));
        }

        // 2. Identifikuj nase postinge
        int my = props.getMyRoutingNumber();
        List<Posting> ours = tx.postings().stream()
                .filter(p -> validator.isOursPerson(p, my))
                .toList();

        // 3. Validate sve nase postinge (PRE REST poziva za rezervaciju)
        List<NoVoteReason> reasons = new ArrayList<>();
        for (Posting p : ours) {
            validatePosting(p, reasons);
        }
        if (!reasons.isEmpty()) {
            return TransactionVote.no(reasons);
        }

        // 4. Reservation sweep — saga-style. Drzimo listu uspesnih ref-ova da
        //    bismo ih oslobodili u catch-u ako sledeci korak padne. REST
        //    pozivi NISU u Spring @Transactional bloku.
        List<Map<String, Object>> committedRefs = new ArrayList<>();
        try {
            for (Posting p : ours) {
                // credit posting (negativan amount) = oduzimanje od naseg balansa = rezervacija
                if (p.amount().signum() < 0) {
                    reservePosting(p, tx.transactionId(), committedRefs);
                }
            }
        } catch (Exception e) {
            log.error("Reservation sweep failed za tx {}; kompenziram {} ref-ova",
                    tx.transactionId(), committedRefs.size(), e);
            compensateReservations(committedRefs);
            throw new InterbankException("Failed to reserve resources", e);
        }

        // 5. Persist PREPARED entity (lokalna @Transactional)
        persistPrepared(tx, committedRefs);
        return TransactionVote.yes();
    }

    /**
     * Per-posting validacija (spec §7.1 step 3 a-g). Punji {@code reasons}
     * listu razloga koji su pronadjeni. Ne baca ekscepcije — vraca razloge
     * kroz out-parametar.
     */
    private void validatePosting(Posting p, List<NoVoteReason> reasons) {
        // c. UNACCEPTABLE_ASSET — STOCK na Account, plus Option-pseudo + ne-OPTION.
        //
        // NAPOMENA (post-handshake fix, 2026-05-12): Person+MONAS NIJE neprihvatljivo
        // per spec §2.6 — Person je opaque foreign-bank-id koji receiver bank
        // resolve-uje u stvarni currency account (po MonetaryAsset.currency). Spec
        // §2.12.1 eksplicitno definise UNACCEPTABLE_ASSET kao "stocks na currency
        // account" tip resniegravnja, ne Person+MONAS. Strict provera bi blokirala
        // legitimno koriscenje Person za cross-bank premium uplate kada coordinator
        // nema otkrivene konkretne account brojeve partnera.
        if (p.account() instanceof TxAccount.Account && p.asset() instanceof Asset.Stock) {
            reasons.add(NoVoteReason.of(Reason.UNACCEPTABLE_ASSET, p));
            return;
        }
        if (p.account() instanceof TxAccount.Option && !(p.asset() instanceof Asset.Option)) {
            reasons.add(NoVoteReason.of(Reason.UNACCEPTABLE_ASSET, p));
            return;
        }

        // a. NO_SUCH_ACCOUNT — za TxAccount.Account proverava resolveAccount
        // d. INSUFFICIENT_ASSET — za credit MONAS posting (negativan amount)
        if (p.account() instanceof TxAccount.Account acc && p.asset() instanceof Asset.Monas) {
            BankingCoreInternalClient.AccountResolveRes res;
            try {
                res = bankingCore.resolveAccount(acc.num());
            } catch (Exception e) {
                reasons.add(NoVoteReason.of(Reason.NO_SUCH_ACCOUNT, p));
                return;
            }
            if (res == null) {
                reasons.add(NoVoteReason.of(Reason.NO_SUCH_ACCOUNT, p));
                return;
            }
            if (p.amount().signum() < 0
                    && res.availableBalance() != null
                    && res.availableBalance().compareTo(p.amount().abs()) < 0) {
                reasons.add(NoVoteReason.of(Reason.INSUFFICIENT_ASSET, p));
                return;
            }
        }

        // e-g. Option validacija
        if (p.asset() instanceof Asset.Option opt) {
            BigDecimal k = BigDecimal.valueOf(opt.asset().amount());
            BigDecimal kPi = k.multiply(opt.asset().pricePerUnit().amount());
            BigDecimal abs = p.amount().abs();
            // Per spec §3.6 (accept flow): OPTION posting amount = 1 (jedan opcioni
            // ugovor se prenosi iz seller-a u buyer-a, asset.amount = k stocks unutar).
            // Per spec §2.7.2 (exercise flow): amount = k (za STOCK transfere)
            // ili k·π (za MONAS transfere kroz option pseudo-account).
            // Pa validan opseg je {1, k, k·π}.
            if (abs.compareTo(BigDecimal.ONE) != 0
                    && abs.compareTo(k) != 0
                    && abs.compareTo(kPi) != 0) {
                reasons.add(NoVoteReason.of(Reason.OPTION_AMOUNT_INCORRECT, p));
                return;
            }
            var negOpt = negRepo.findById(opt.asset().negotiationId().id());
            if (negOpt.isEmpty()) {
                reasons.add(NoVoteReason.of(Reason.OPTION_NEGOTIATION_NOT_FOUND, p));
                return;
            }
            var neg = negOpt.get();
            // OTC opcija "iskoristiva" je samo ako je negotiation jos ongoing
            // (autoritativno-mirror flag-uje "ACTIVE" pregovor) i ako
            // settlement datum nije prosao. spec §7.1 (f).
            if (!neg.isOngoing()
                    || (neg.getSettlementDate() != null
                            && neg.getSettlementDate().isBefore(OffsetDateTime.now()))) {
                reasons.add(NoVoteReason.of(Reason.OPTION_USED_OR_EXPIRED, p));
            }
        }
    }

    /**
     * Dispatch za rezervaciju jednog postinga ka banking-core (MONAS) ili
     * trading-u (STOCK/OPTION). Dodaje {@link #REF_TYPE}/{@link #REF_ID} map u
     * {@code committed} listu da bi kompenzacija znala sta da oslobodi.
     */
    private void reservePosting(Posting p, ForeignBankId txId, List<Map<String, Object>> committed) {
        if (p.asset() instanceof Asset.Monas m && p.account() instanceof TxAccount.Account acc) {
            var res = bankingCore.reserveMonas(new BankingCoreInternalClient.ReserveMonasReq(
                    acc.num(), m.asset().currency(), p.amount().abs(),
                    txId.routingNumber(), txId.id()));
            committed.add(refMap("MONAS", res.reservationId().toString()));
        } else if (p.asset() instanceof Asset.Stock s && p.account() instanceof TxAccount.Person per) {
            // Person.id.id u nasoj banci je foreign ID (per Tim 2 §3.2 koristi
            // "C-N" ili "E-N" prefix). Trading-service interni sellerUserId je
            // goli Long, pa strpamo prefiks pre parsiranja.
            String foreignId = per.id().id();
            String numericPart = foreignId;
            if (foreignId != null && (foreignId.startsWith("C-") || foreignId.startsWith("E-"))) {
                numericPart = foreignId.substring(2);
            }
            Long sellerUserId;
            try {
                sellerUserId = Long.valueOf(numericPart);
            } catch (NumberFormatException ex) {
                throw new InterbankException(
                        "Stock reservation seller ID nije Long ni 'C-N'/'E-N': " + foreignId, ex);
            }
            var res = trading.reserveStock(new TradingInternalClient.ReserveStockReq(
                    sellerUserId, s.asset().ticker(), p.amount().abs().intValue(),
                    txId.routingNumber(), txId.id()));
            committed.add(refMap("STOCK", res.reservationId().toString()));
        } else if (p.asset() instanceof Asset.Option opt && p.account() instanceof TxAccount.Option pseudo) {
            // Resolve seller (vlasnik option pseudo-account-a) iz negotiation mirror-a
            // umesto da pretpostavlja da pseudo.id() drzi seller. Per Tim 2 §3.6,
            // option pseudo-account.id IS negotiationId — ne sadrzi info o seller-u.
            // Seller's foreignId zivi u interbank_negotiations.seller_* poljima.
            String negId = opt.asset().negotiationId().id();
            var negOpt = negRepo.findById(negId);
            if (negOpt.isEmpty()) {
                throw new InterbankException("Pre OPTION reservation, negotiation " + negId
                        + " mora postojati u nasoj DB (lookup u prepareLocal validation)");
            }
            String sellerForeignId = negOpt.get().getSellerId();
            trading.reserveOption(
                    negId,
                    sellerForeignId,
                    opt.asset().stock().ticker(),
                    opt.asset().amount());
            committed.add(refMap("OPTION", negId));
        }
    }

    /**
     * Iterira committed ref-ove unazad i pokusava da svaki oslobodi. Greska na
     * pojedinacnom ref-u se loguje ali ne prekida sweep — preostali ref-ovi
     * moraju da imaju sansu da se oslobode.
     */
    private void compensateReservations(List<Map<String, Object>> refs) {
        for (int i = refs.size() - 1; i >= 0; i--) {
            Map<String, Object> ref = refs.get(i);
            try {
                String type = (String) ref.get(REF_TYPE);
                String id = (String) ref.get(REF_ID);
                switch (type) {
                    case "MONAS" -> bankingCore.releaseMonas(UUID.fromString(id));
                    case "STOCK" -> trading.releaseStock(UUID.fromString(id));
                    case "OPTION" -> trading.releaseOption(id);
                    default -> log.warn("Nepoznat ref tip {}", type);
                }
            } catch (Exception e) {
                log.error("Stuck reservation: {}", ref, e);
            }
        }
    }

    /**
     * Persist PREPARED entity. Posebna metoda da bi se izolovao Spring
     * {@code @Transactional} boundary samo na lokalnu DB. Ako save fail-uje,
     * sve vec uspesne rezervacije se kompenzuju da bismo bili u stanju da
     * vratimo grešku ka pozivocu bez "stuck" rezervacija.
     */
    @Transactional
    public void persistPrepared(InterbankTransactionPayload tx, List<Map<String, Object>> refs) {
        try {
            Map<String, Object> meta = new LinkedHashMap<>();
            meta.put("message", tx.message());
            meta.put("callNumber", tx.callNumber());
            meta.put("paymentCode", tx.paymentCode());
            meta.put("paymentPurpose", tx.paymentPurpose());

            InterbankTransactionEntity entity = InterbankTransactionEntity.builder()
                    .transactionIdRouting(tx.transactionId().routingNumber())
                    .transactionIdLocal(tx.transactionId().id())
                    .status(TxStatus.PREPARED)
                    .postingsJson(mapper.writeValueAsString(tx.postings()))
                    .reservationRefs(mapper.writeValueAsString(refs))
                    .messageMeta(mapper.writeValueAsString(meta))
                    .build();
            txRepo.save(entity);
        } catch (Exception e) {
            log.error("persistPrepared failed za tx {}; kompenziram rezervacije",
                    tx.transactionId(), e);
            compensateReservations(refs);
            throw new InterbankException("Failed to persist prepared transaction", e);
        }
    }

    /**
     * COMMIT_TX faza per spec §7.2. Idempotent:
     * <ul>
     *   <li>Ako transakcija ne postoji — log warn i no-op (mozda commit poruka
     *       stigla a mi nikad nismo videli prepare; partner banka je gospodar
     *       protokola, ne nas).</li>
     *   <li>Ako je vec COMMITTED — no-op.</li>
     *   <li>Ako je u terminalnom stanju razlicitom od COMMITTED (ROLLED_BACK /
     *       FAILED) — baca {@link AlreadyCommittedException}.</li>
     * </ul>
     */
    @Transactional
    public void commitLocal(ForeignBankId txId) {
        var txOpt = txRepo.findByTransactionIdRoutingAndTransactionIdLocal(
                txId.routingNumber(), txId.id());
        if (txOpt.isEmpty()) {
            log.warn("commitLocal za nepoznatu tx {} — idempotent no-op", txId);
            return;
        }
        var tx = txOpt.get();
        if (tx.getStatus() == TxStatus.COMMITTED) {
            log.debug("Idempotent commit {}", txId);
            return;
        }
        if (tx.getStatus() != TxStatus.PREPARED) {
            throw new AlreadyCommittedException(
                    "Cannot commit transakciju iz stanja " + tx.getStatus());
        }

        try {
            List<Map<String, Object>> refs = parseRefs(tx.getReservationRefs());
            for (Map<String, Object> ref : refs) {
                String type = (String) ref.get(REF_TYPE);
                String id = (String) ref.get(REF_ID);
                switch (type) {
                    case "MONAS" -> bankingCore.commitMonas(UUID.fromString(id));
                    case "STOCK" -> trading.commitStock(UUID.fromString(id));
                    case "OPTION" -> trading.exerciseOption(id);
                    default -> log.warn("Nepoznat ref tip pri commit-u: {}", type);
                }
            }
            tx.setStatus(TxStatus.COMMITTED);
            tx.setFinalizedAt(Instant.now());
            txRepo.save(tx);
        } catch (AlreadyCommittedException e) {
            throw e;
        } catch (Exception e) {
            tx.setStatus(TxStatus.FAILED);
            txRepo.save(tx);
            throw new InterbankException("commitLocal failed", e);
        }
    }

    /**
     * ROLLBACK_TX faza per spec §7.3. Idempotent slicno kao commit:
     * <ul>
     *   <li>Tx ne postoji — no-op.</li>
     *   <li>Vec ROLLED_BACK — no-op.</li>
     *   <li>Terminalno stanje razlicito (COMMITTED / FAILED) — no-op uz log
     *       warn. NE bacamo ekscepciju jer protokol mora da bude "best effort"
     *       da se sistem ne zaglavi.</li>
     * </ul>
     */
    @Transactional
    public void rollbackLocal(ForeignBankId txId) {
        var txOpt = txRepo.findByTransactionIdRoutingAndTransactionIdLocal(
                txId.routingNumber(), txId.id());
        if (txOpt.isEmpty()) {
            log.warn("rollbackLocal za nepoznatu tx {} — idempotent no-op", txId);
            return;
        }
        var tx = txOpt.get();
        if (tx.getStatus() == TxStatus.ROLLED_BACK) {
            log.debug("Idempotent rollback {}", txId);
            return;
        }
        if (tx.getStatus() != TxStatus.PREPARED) {
            log.warn("rollbackLocal za tx {} u stanju {} — ne radim nista", txId, tx.getStatus());
            return;
        }

        try {
            List<Map<String, Object>> refs = parseRefs(tx.getReservationRefs());
            compensateReservations(refs);
            tx.setStatus(TxStatus.ROLLED_BACK);
            tx.setFinalizedAt(Instant.now());
            txRepo.save(tx);
        } catch (Exception e) {
            tx.setStatus(TxStatus.FAILED);
            txRepo.save(tx);
            throw new InterbankException("rollbackLocal failed", e);
        }
    }

    private List<Map<String, Object>> parseRefs(String json) {
        if (json == null || json.isBlank()) {
            return List.of();
        }
        try {
            return mapper.readValue(json, new TypeReference<List<Map<String, Object>>>() {});
        } catch (Exception e) {
            throw new InterbankException("Failed to parse reservation_refs JSON", e);
        }
    }

    private static Map<String, Object> refMap(String type, String id) {
        Map<String, Object> m = new LinkedHashMap<>(2);
        m.put(REF_TYPE, type);
        m.put(REF_ID, id);
        return m;
    }
}
