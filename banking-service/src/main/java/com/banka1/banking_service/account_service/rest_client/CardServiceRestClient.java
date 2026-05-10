//package com.banka1.banking_service.account_service.rest_client;
//
//import com.banka1.banking_service.account_service.dto.request.CreateCardRequestDto;
//import com.banka1.banking_service.account_service.dto.response.CardResponseDto;
//import lombok.extern.slf4j.Slf4j;
//import org.springframework.beans.factory.annotation.Qualifier;
//import org.springframework.core.ParameterizedTypeReference;
//import org.springframework.stereotype.Service;
//import org.springframework.web.client.RestClient;
//import org.springframework.web.client.RestClientException;
//
//import java.util.List;
//
//@Slf4j
//@Service
//public class CardServiceRestClient {
//
//    private final RestClient cardRestClient;
//
//    public CardServiceRestClient(@Qualifier("cardRestClient") RestClient cardRestClient) {
//        this.cardRestClient = cardRestClient;
//    }
//
//    public List<CardResponseDto> getCardsForAccount(String accountNumber) {
//        try {
//            List<CardResponseDto> cards = cardRestClient.get()
//                    .uri("/internal/account/{accountNumber}", accountNumber)
//                    .retrieve()
//                    .body(new ParameterizedTypeReference<List<CardResponseDto>>() {});
//            return cards != null ? cards : List.of();
//        } catch (RestClientException e) {
//            log.warn("Card service unavailable for account {}: {}", accountNumber, e.getMessage());
//            return List.of();
//        }
//    }
//
//    public boolean createCardForNewAccount(CreateCardRequestDto requestDto) {
//        try {
//            cardRestClient.post()
//                    .uri("/cards/auto")
//                    .body(requestDto)
//                    .retrieve()
//                    .toBodilessEntity();
//            return true;
//        } catch (RestClientException e) {
//            log.error("Failed to create card for account {}: {}", requestDto.getAccountNumber(), e.getMessage());
//            return false;
//        }
//    }
//}
