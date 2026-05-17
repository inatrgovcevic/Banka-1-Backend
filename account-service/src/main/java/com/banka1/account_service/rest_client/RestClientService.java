package com.banka1.account_service.rest_client;

import com.banka1.account_service.dto.response.ClientInfoResponseDto;
import lombok.RequiredArgsConstructor;
import org.springframework.beans.factory.annotation.Qualifier;
import org.springframework.stereotype.Service;
import org.springframework.web.client.RestClient;

@Service

public class RestClientService {

    private final RestClient restClient;


    public RestClientService(@Qualifier("userRestClient") RestClient restClient) {
        this.restClient = restClient;
    }

    public ClientInfoResponseDto getUser(String jmbg) {
        return restClient.get()
                .uri("/clients/customers/jmbg/{jmbg}", jmbg)
                .retrieve()
                .body(ClientInfoResponseDto.class);
    }
    public ClientInfoResponseDto getUser(Long id) {
        return restClient.get()
                .uri("/clients/customers/{id}", id)
                .retrieve()
                .body(ClientInfoResponseDto.class);
    }

}
