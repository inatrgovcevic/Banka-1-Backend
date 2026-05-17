package com.banka1.credit_service.rest_client;

import org.springframework.beans.factory.annotation.Qualifier;
import org.springframework.stereotype.Service;
import org.springframework.web.client.RestClient;

/**
 * REST Client for communicating with the Client/User Service.
 * Provides methods for retrieving client information.
 */
@Service
public class ClientService {


    private final RestClient restClient;

    /**
     * Constructs ClientService with a qualified RestClient bean.
     *
     * @param restClient the RestClient bean configured for client service communication
     */
    public ClientService(@Qualifier("userClient") RestClient restClient) {
        this.restClient = restClient;
    }


    public void addMarginPermission(Long id)
    {
        restClient.put().uri("/clients/customers/margin/{id}",id).retrieve().toBodilessEntity();
    }

//    public ClientInfoResponseDto getUser(String jmbg) {
//        return clientServiceClient.get()
//                .uri("/clients/customers/jmbg/{jmbg}", jmbg)
//                .retrieve()
//                .body(ClientInfoResponseDto.class);
//    }
//    public ClientInfoResponseDto getUser(Long id) {
//        return clientServiceClient.get()
//                .uri("/clients/customers/{id}", id)
//                .retrieve()
//                .body(ClientInfoResponseDto.class);
//    }

}
