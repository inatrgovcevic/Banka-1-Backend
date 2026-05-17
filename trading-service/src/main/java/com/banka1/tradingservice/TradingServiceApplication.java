package com.banka1.tradingservice;

import org.springframework.boot.SpringApplication;
import org.springframework.boot.autoconfigure.SpringBootApplication;
import org.springframework.boot.context.properties.ConfigurationPropertiesScan;
import org.springframework.boot.persistence.autoconfigure.EntityScan;
import org.springframework.context.annotation.ComponentScan;
import org.springframework.context.annotation.FilterType;
import org.springframework.context.annotation.FullyQualifiedAnnotationBeanNameGenerator;
import org.springframework.data.jpa.repository.config.EnableJpaRepositories;
import org.springframework.scheduling.annotation.EnableScheduling;

/**
 * PR_19 C19.X: konsolidovani trading-service — ucita order-service legacy
 * module kao project() dep i scan-uje sve {@code com.banka1} pakete tako da
 * order/portfolio/tax/actuary controlleri zive u istoj JVM instanci pored
 * trading-service margin/OTC/funds koda.
 */
@SpringBootApplication(nameGenerator = FullyQualifiedAnnotationBeanNameGenerator.class)
@ConfigurationPropertiesScan(basePackages = "com.banka1")
@EnableScheduling
@ComponentScan(
        basePackages = {"com.banka1"},
        nameGenerator = FullyQualifiedAnnotationBeanNameGenerator.class,
        excludeFilters = {
                @ComponentScan.Filter(type = FilterType.REGEX, pattern = ".*\\.OrderServiceApplication"),
                @ComponentScan.Filter(type = FilterType.REGEX, pattern = ".*\\.UserServiceApplication"),
                @ComponentScan.Filter(type = FilterType.REGEX, pattern = ".*\\.BankingCoreServiceApplication"),
                @ComponentScan.Filter(type = FilterType.REGEX, pattern = ".*\\.MarketServiceApplication")
        }
)
@EntityScan(basePackages = {
        "com.banka1.tradingservice",
        "com.banka1.order"
})
@EnableJpaRepositories(basePackages = {
        "com.banka1.tradingservice",
        "com.banka1.order"
})
public class TradingServiceApplication {

    public static void main(String[] args) {
        SpringApplication.run(TradingServiceApplication.class, args);
    }
}
