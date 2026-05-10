package com.banka1.banking_service.account_service.scheduler;

import com.banka1.banking_service.account_service.repository.AccountRepository;
import com.banka1.banking_service.account_service.service.MaintenanceFeeService;
import lombok.RequiredArgsConstructor;
import lombok.extern.slf4j.Slf4j;
import org.springframework.scheduling.annotation.Scheduled;
import org.springframework.stereotype.Component;
import org.springframework.transaction.annotation.Transactional;

/**
 * Komponent za zakazane poslove u Account servisu.
 * <p>
 * Odgovoran je za:
 * <ul>
 *   <li><strong>Dnevni reset</strong> - resetovanje dnevnog trošenja na 0 svaki dan u ponoć</li>
 *   <li><strong>Mesečni reset</strong> - resetovanje mesečnog trošenja na 0 prvog u mesecu</li>
 *   <li><strong>Mesečne naknade</strong> - obrada mesečnih naknada za održavanje tekućih računa</li>
 * </ul>
 * <p>
 * Svi poslovi se izvršavaju transakciono kako bi se osigurala konzistentnost podataka.
 */
@Slf4j
@Component
@RequiredArgsConstructor
public class Scheduler {

    private final AccountRepository accountRepository;
    private final MaintenanceFeeService maintenanceFeeService;

    /**
     * Zakazani posao koji se pokreće svaki dan u ponoć (00:00:00).
     * <p>
     * Resetuje dnevno trošenje (<code>dnevnaPotrosnja</code>) na 0 za sve račune.
     * To omogućava da korisnici ponovo mogu trošiti do svoje dnevne limite
     * sledećeg dana.
     * <p>
     * <strong>Raspored:</strong> Cron: {@code "0 0 0 * * *"}
     */
    @Scheduled(cron = "0 0 0 * * *")
    @Transactional
    public void resetDailySpending() {
        int updated = accountRepository.resetDailySpending();
        log.info("Daily spending reset executed. Updated accounts: {}", updated);
    }

    /**
     * Zakazani posao koji se pokreće prvog u mesecu u ponoć (00:00:00).
     * <p>
     * Resetuje mesečno trošenje (<code>mesecnaPotrosnja</code>) na 0 za sve račune.
     * To omogućava da korisnici ponovo mogu trošiti do svoje mesečne limite
     * sledećeg meseca.
     * <p>
     * <strong>Raspored:</strong> Cron: {@code "0 0 0 1 * *"}
     */
    @Scheduled(cron = "0 0 0 1 * *")
    @Transactional
    public void resetMonthlySpending() {
        int updated = accountRepository.resetMonthlySpending();
        log.info("Monthly spending reset executed. Updated accounts: {}", updated);
    }

    /**
     * Zakazani posao koji se pokreće prvog u mesecu u ponoć (00:00:00).
     * <p>
     * Obrađuje mesečne naknade za održavanje tekućih računa. Naknade se
     * oduzimaju od stanja računa.
     * <p>
     * <strong>Raspored:</strong> Cron: {@code "0 0 0 1 * *"}
     */
    @Scheduled(cron = "0 0 0 1 * *")
    public void run() {
        log.info("Starting monthly maintenance fee job");
        maintenanceFeeService.process();
        log.info("Finished monthly maintenance fee job");
    }

}
