package com.banka1.marketservice;

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
 * PR_19 C19.X: konsolidovani market-service — ucita stock + exchange legacy
 * module kao project() deps i scan-uje sve {@code com.banka1} pakete tako da
 * REST controlleri (/stocks/*, /exchange/*) zive u istoj JVM instanci.
 */
@SpringBootApplication(nameGenerator = FullyQualifiedAnnotationBeanNameGenerator.class)
@ConfigurationPropertiesScan(basePackages = "com.banka1")
@EnableScheduling
@ComponentScan(
        basePackages = {"com.banka1"},
        nameGenerator = FullyQualifiedAnnotationBeanNameGenerator.class,
        excludeFilters = {
                @ComponentScan.Filter(type = FilterType.REGEX, pattern = ".*\\.StockServiceApplication"),
                @ComponentScan.Filter(type = FilterType.REGEX, pattern = ".*\\.ExchangeServiceApplication"),
                @ComponentScan.Filter(type = FilterType.REGEX, pattern = ".*\\.UserServiceApplication"),
                @ComponentScan.Filter(type = FilterType.REGEX, pattern = ".*\\.BankingCoreServiceApplication"),
                @ComponentScan.Filter(type = FilterType.REGEX, pattern = ".*\\.TradingServiceApplication")
        }
)
@EntityScan(basePackages = {
        "com.banka1.marketservice",
        "com.banka1.stock_service",
        "com.banka1.exchangeService"
})
@EnableJpaRepositories(basePackages = {
        "com.banka1.marketservice",
        "com.banka1.stock_service",
        "com.banka1.exchangeService"
})
public class MarketServiceApplication {

    public static void main(String[] args) {
        SpringApplication.run(MarketServiceApplication.class, args);
    }
}
