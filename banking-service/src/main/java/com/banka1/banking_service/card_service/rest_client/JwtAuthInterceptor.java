package com.banka1.banking_service.card_service.rest_client;

import com.banka1.banking_service.card_service.security.JWTService;
import lombok.RequiredArgsConstructor;
import org.springframework.http.HttpRequest;
import org.springframework.http.client.ClientHttpRequestExecution;
import org.springframework.http.client.ClientHttpRequestInterceptor;
import org.springframework.http.client.ClientHttpResponse;

import java.io.IOException;

/**
 * Adds a service JWT to outgoing inter-service HTTP requests.
 */
@RequiredArgsConstructor
public class JwtAuthInterceptor implements ClientHttpRequestInterceptor {

    private final JWTService jwtService;

    @Override
    public ClientHttpResponse intercept(
            HttpRequest request,
            byte[] body,
            ClientHttpRequestExecution execution
    ) throws IOException {
        request.getHeaders().setBearerAuth(jwtService.generateJwtToken());
        return execution.execute(request, body);
    }
}
