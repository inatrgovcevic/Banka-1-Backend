package com.banka1.clientService.filter;

import org.springframework.beans.factory.annotation.Value;
import org.springframework.boot.web.servlet.FilterRegistrationBean;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;

/**
 * Spring konfiguracija koja registruje {@link RateLimitFilter} za odredjene URL putanje.
 * Filter se primenjuje sa najvisom prioritetom (order = 1) kako bi se rate limit proverio
 * pre ostalih filtera u lancu.
 * Parametri se citaju iz application.properties kako bi mogli biti podesavani bez rebuild-a.
 */
@Configuration("clientRateLimitConfig")
public class RateLimitConfig {

    @Value("${rate-limit.max-requests:10}")
    private int maxRequests;

    @Value("${rate-limit.window-ms:60000}")
    private long windowMs;

    /**
     * Registruje {@link RateLimitFilter} i ogranicava ga na POST /customers endpoint.
     *
     * @return konfigurisan bean registracije filtera
     */
    @Bean(name = "clientRateLimitFilter")
    public FilterRegistrationBean<RateLimitFilter> rateLimitFilter() {
        FilterRegistrationBean<RateLimitFilter> registration = new FilterRegistrationBean<>();
        registration.setFilter(new RateLimitFilter(maxRequests, windowMs));
        registration.addUrlPatterns("/clients/customers");
        registration.setOrder(1);
        return registration;
    }
}
