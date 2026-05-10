package com.banka1.banking_service.credit_service.rest_client;

import com.banka1.banking_service.credit_service.security.JWTService;
import lombok.RequiredArgsConstructor;
import org.springframework.context.annotation.Configuration;
import org.springframework.http.HttpRequest;
import org.springframework.http.client.ClientHttpRequestExecution;
import org.springframework.http.client.ClientHttpRequestInterceptor;
import org.springframework.http.client.ClientHttpResponse;

import java.io.IOException;


/**
 * HTTP request interceptor that adds JWT bearer token authentication to all outgoing REST requests.
 * Automatically injects the Authorization header with a generated JWT token.
 */
@Configuration
@RequiredArgsConstructor
public class JwtAuthInterceptor implements ClientHttpRequestInterceptor {

    private final JWTService jwtProvider;

    /**
     * Intercepts HTTP requests and adds JWT authentication header.
     * Retrieves a JWT token from JWTService and adds it as a Bearer token
     * in the Authorization header before executing the request.
     *
     * @param request the HTTP request being executed
     * @param body the request body bytes
     * @param execution the execution handler for the request
     * @return the HTTP response
     * @throws IOException if an I/O error occurs
     */
    @Override
    public ClientHttpResponse intercept(HttpRequest request, byte[] body,
                                        ClientHttpRequestExecution execution) throws IOException {

        String token = jwtProvider.generateJwtToken();
        request.getHeaders().set("Authorization", "Bearer " + token);
        return execution.execute(request, body);
    }
}
