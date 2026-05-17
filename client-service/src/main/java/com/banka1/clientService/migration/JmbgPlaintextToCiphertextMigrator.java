package com.banka1.clientService.migration;

import com.banka1.security.crypto.JmbgEncryptor;
import lombok.RequiredArgsConstructor;
import lombok.extern.slf4j.Slf4j;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.boot.context.event.ApplicationReadyEvent;
import org.springframework.context.event.EventListener;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.stereotype.Component;
import org.springframework.transaction.annotation.Transactional;

import java.util.List;
import java.util.Map;

/**
 * One-shot migrator za enkripciju postojecih plaintext JMBG-ova posle
 * Liquibase changeset-a {@code 008-encrypt-jmbg.sql} (PR_07 C7.1) koji je dodao
 * {@code jmbg_encrypted} kolonu (PR_11 C11.12 real implementacija).
 *
 * <p>Strategija:
 * <ol>
 *   <li>Liquibase 008 dodaje {@code jmbg_encrypted} kolonu kao TEXT.
 *   <li>Ovaj migrator se aktivira na {@link ApplicationReadyEvent} (posle Spring
 *       context-a + Liquibase migracija).
 *   <li>SELECT id, jmbg FROM clients WHERE jmbg IS NOT NULL AND jmbg_encrypted IS NULL.
 *   <li>Za svaki red, enkriptuj plaintext JMBG sa {@link JmbgEncryptor} i UPDATE.
 *   <li>Migrator se izvrsava samo ako je {@code banka.security.jmbg-migrator-enabled=true}
 *       (default false). Aktivira se eksplicitno u prod deploy-u jednom, posle cega se isključuje.
 *   <li>Liquibase 009 (drop plaintext jmbg kolone) ima precondition: count plaintext
 *       gde encrypted IS NULL = 0. Ako je ovaj migrator zavrsio posao, condition
 *       prolazi i kolona se uklanja.
 * </ol>
 *
 * <p>Idempotentnost: WHERE jmbg_encrypted IS NULL kondicija sprecava re-encryption
 * pri ponovnom startup-u.
 */
@Slf4j
@Component
@RequiredArgsConstructor
public class JmbgPlaintextToCiphertextMigrator {

    private final JdbcTemplate jdbcTemplate;
    private final JmbgEncryptor encryptor;

    @Value("${banka.security.jmbg-migrator-enabled:false}")
    private boolean enabled;

    @EventListener(ApplicationReadyEvent.class)
    @Transactional
    public void migrate() {
        if (!enabled) {
            log.debug("JMBG migrator nije aktiviran (BANKA_SECURITY_JMBG_MIGRATOR_ENABLED=false). Skip.");
            return;
        }

        // Provera da kolona jmbg postoji (mozda je vec drop-ovana u changeset 009).
        Integer columnCount = jdbcTemplate.queryForObject(
                "SELECT COUNT(*) FROM information_schema.columns " +
                        "WHERE table_name = 'clients' AND column_name = 'jmbg'",
                Integer.class);
        if (columnCount == null || columnCount == 0) {
            log.info("JMBG migrator: 'jmbg' kolona vise ne postoji (vec dropovana). Skip.");
            return;
        }

        List<Map<String, Object>> rows = jdbcTemplate.queryForList(
                "SELECT id, jmbg FROM clients WHERE jmbg IS NOT NULL AND jmbg_encrypted IS NULL"
        );
        log.info("JMBG migrator: pronadjeno {} redova za enkripciju", rows.size());

        int success = 0, failed = 0;
        for (Map<String, Object> row : rows) {
            Long id = ((Number) row.get("id")).longValue();
            String plaintext = (String) row.get("jmbg");
            try {
                String ciphertext = encryptor.encrypt(plaintext);
                jdbcTemplate.update(
                        "UPDATE clients SET jmbg_encrypted = ? WHERE id = ?",
                        ciphertext, id
                );
                success++;
            } catch (Exception ex) {
                log.error("JMBG migrator: failed for clientId={} ({}). Skip.", id, ex.toString());
                failed++;
            }
        }
        log.info("JMBG migrator zavrsen: success={} failed={}", success, failed);

        if (failed > 0) {
            log.warn("JMBG migrator: {} redova nije migrirao — Liquibase 009 (drop jmbg) NECE proci precondition. " +
                            "Operativac mora rucno proveriti i ponoviti.", failed);
        }
    }
}
