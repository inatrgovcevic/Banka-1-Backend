package com.banka1.interbank;

import org.springframework.boot.SpringApplication;
import org.springframework.boot.autoconfigure.SpringBootApplication;
import org.springframework.boot.context.properties.ConfigurationPropertiesScan;
import org.springframework.scheduling.annotation.EnableScheduling;

/**
 * PR_32 Phase 1: main application za interbank-service.
 *
 * scanBasePackages = "com.banka1" zato sto security-lib i
 * company-observability-starter zive u com.banka1.* paketima i moraju biti
 * pokupljeni kroz @ComponentScan / auto-config.
 *
 * @EnableScheduling potreban za buduce schedule-ovane task-ove (npr. expiry
 * job za interbank kontrakte u Phase 6+).
 */
@SpringBootApplication(scanBasePackages = "com.banka1")
@ConfigurationPropertiesScan
@EnableScheduling
public class InterbankServiceApplication {

    public static void main(String[] args) {
        SpringApplication.run(InterbankServiceApplication.class, args);
    }
}
