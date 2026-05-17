package com.banka1.userservice;

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
 * PR_19 C19.X: konsolidovani user-service — ucita employee + client legacy module
 * kao project() deps i scan-uje sve {@code com.banka1} pakete tako da REST controlleri
 * iz {@code com.banka1.employeeService} i {@code com.banka1.clientService} budu
 * registrovani u istoj JVM instanci.
 *
 * <p>Entity scan se konfigurise kroz {@code spring.jpa.entity-scan-base-packages}
 * u application.properties (Spring Boot 4 je uklonio @EntityScan annotation).
 */
@SpringBootApplication(nameGenerator = FullyQualifiedAnnotationBeanNameGenerator.class)
@ConfigurationPropertiesScan(basePackages = "com.banka1")
@EnableScheduling
@ComponentScan(
        basePackages = {"com.banka1"},
        nameGenerator = FullyQualifiedAnnotationBeanNameGenerator.class,
        excludeFilters = {
                @ComponentScan.Filter(
                        type = FilterType.REGEX,
                        pattern = ".*\\.EmployeeServiceApplication"),
                @ComponentScan.Filter(
                        type = FilterType.REGEX,
                        pattern = ".*\\.ClientServiceApplication")
        }
)
@EntityScan(basePackages = {
        "com.banka1.userservice",
        "com.banka1.employeeService",
        "com.banka1.clientService"
})
@EnableJpaRepositories(basePackages = {
        "com.banka1.userservice",
        "com.banka1.employeeService",
        "com.banka1.clientService"
})
public class UserServiceApplication {

    public static void main(String[] args) {
        SpringApplication.run(UserServiceApplication.class, args);
    }
}
