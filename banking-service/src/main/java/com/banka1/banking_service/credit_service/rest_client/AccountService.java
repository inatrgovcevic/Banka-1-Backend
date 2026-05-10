package com.banka1.banking_service.credit_service.rest_client;

import com.banka1.banking_service.credit_service.dto.request.BankPaymentDto;
import com.banka1.banking_service.credit_service.dto.request.PaymentDto;
import com.banka1.banking_service.credit_service.dto.response.AccountDetailsResponseDto;
import com.banka1.banking_service.credit_service.dto.response.InfoResponseDto;
import com.banka1.banking_service.credit_service.dto.response.UpdatedBalanceResponseDto;
import org.springframework.beans.factory.annotation.Qualifier;
import org.springframework.stereotype.Service;
import org.springframework.web.client.RestClient;

/**
 * REST Client for communicating with the Account Service.
 * Provides methods for account transfers, transactions, and account details retrieval.
 */
@Service
public class AccountService {

    private final RestClient restClient;

    /**
     * Constructs AccountService with a qualified RestClient bean.
     *
     * @param restClient the RestClient bean configured for account service communication
     */
    public AccountService(@Qualifier("creditAccountClient") RestClient restClient) {
        this.restClient = restClient;
    }

    /**
     * Retrieves account information for both source and destination accounts.
     *
     * @param fromBankNumber the source bank account number
     * @param toBankNumber the destination bank account number
     * @return InfoResponseDto containing account details for both accounts
     */
    public InfoResponseDto getInfo(String fromBankNumber, String toBankNumber) {
        return restClient.get()
                .uri(uriBuilder -> uriBuilder
                        .path("/internal/accounts/info")
                        .queryParam("fromBankNumber", fromBankNumber)
                        .queryParam("toBankNumber", toBankNumber)
                        .build())
                .retrieve()
                .body(InfoResponseDto.class);
    }

    /**
     * Performs a transfer between accounts.
     *
     * @param paymentDto the transfer details
     * @return UpdatedBalanceResponseDto with updated balance information
     */
    public UpdatedBalanceResponseDto transfer(PaymentDto paymentDto) {
        return restClient.post()
                .uri("/internal/accounts/transfer")
                .body(paymentDto)
                .retrieve()
                .body(UpdatedBalanceResponseDto.class);
    }

    /**
     * Performs a transaction between accounts.
     *
     * @param paymentDto the transaction details
     * @return UpdatedBalanceResponseDto with updated balance information
     */
    public UpdatedBalanceResponseDto transaction(PaymentDto paymentDto) {
        return restClient.post()
                .uri("/internal/accounts/transaction")
                .body(paymentDto)
                .retrieve()
                .body(UpdatedBalanceResponseDto.class);
    }

    /**
     * Performs a transaction originating from the bank.
     *
     * @param paymentDto the bank payment details
     * @return UpdatedBalanceResponseDto with updated balance information
     */
    public UpdatedBalanceResponseDto transactionFromBank(BankPaymentDto paymentDto) {
        return restClient.post()
                .uri("/internal/accounts/transactionFromBank")
                .body(paymentDto)
                .retrieve()
                .body(UpdatedBalanceResponseDto.class);
    }

    /**
     * Retrieves detailed information about a specific account.
     *
     * @param accountNumber the account number
     * @return AccountDetailsResponseDto with account details
     */
    public AccountDetailsResponseDto getDetails(String accountNumber)
    {
        return restClient.get()
                .uri("/internal/accounts/{accountNumber}/details", accountNumber)
                .retrieve()
                .body(AccountDetailsResponseDto.class);
    }
}
