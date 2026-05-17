package com.banka1.employeeService.filter;

import org.springframework.boot.web.servlet.FilterRegistrationBean;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;

/**
 * Spring konfiguracija koja registruje {@link RateLimitFilter} za odredjene URL putanje.
 * Filter se primenjuje sa najvisom prioritetom (order = 1) kako bi se rate limit proverio
 * pre ostalih filtera u lancu.
 */
@Configuration("employeeRateLimitConfig")
public class RateLimitConfig {

    /**
     * Registruje {@link RateLimitFilter} i ogranicava ga na osetljive auth endpoint-e.
     *
     * @return konfigurisan bean registracije filtera
     */
    @Bean(name = "employeeRateLimitFilter")
    public FilterRegistrationBean<RateLimitFilter> rateLimitFilter() {
        FilterRegistrationBean<RateLimitFilter> registration = new FilterRegistrationBean<>();
        registration.setFilter(new RateLimitFilter());
        registration.addUrlPatterns("/auth/login", "/auth/forgot-password", "/auth/refresh");
        registration.setOrder(1);
        return registration;
    }
}
