package com.banka1.banking_service.transfer_service.client.impl;

import com.banka1.banking_service.transfer_service.client.AccountClient;
import com.banka1.banking_service.transfer_service.dto.client.AccountDto;
import com.banka1.banking_service.account_service.dto.request.PaymentDto;
import com.banka1.banking_service.transfer_service.dto.client.UpdatedBalanceResponseDto;
import com.banka1.banking_service.transfer_service.exception.BusinessException;
import com.banka1.banking_service.transfer_service.exception.ErrorCode;
import lombok.RequiredArgsConstructor;
import lombok.extern.slf4j.Slf4j;
import org.springframework.context.annotation.Profile;
import org.springframework.stereotype.Component;
import org.springframework.web.client.HttpClientErrorException;
import org.springframework.web.client.RestClient;

/**
 * Implementacija klijenta za Account Service koja koristi RestClient.
 * Aktivna samo u profilima koji nisu "local".
 */
@Component
@Profile("!local")
@RequiredArgsConstructor
@Slf4j
public class AccountClientImpl implements AccountClient {

    private final RestClient accountRestClient;

    @Override
    public AccountDto getAccountDetails(String accountNumber) {
        try {
            return accountRestClient.get()
                    .uri("/internal/accounts/{accountNumber}/details", accountNumber)
                    .retrieve()
                    .body(AccountDto.class);
        } catch (HttpClientErrorException.NotFound e) {
            throw new BusinessException(ErrorCode.ACCOUNT_NOT_FOUND, "Račun " + accountNumber + " ne postoji.");
        } catch (Exception e) {
            log.error("Account service error for account {}: {}", accountNumber, e.getMessage());
            throw new BusinessException(ErrorCode.TRANSFER_NOT_FOUND, "Servis računa nije dostupan.");
        }
    }

    @Override
    public UpdatedBalanceResponseDto executeTransfer(PaymentDto paymentDto) {
        try {
            return accountRestClient.post()
                    .uri("/internal/accounts/transfer")
                    .body(paymentDto)
                    .retrieve()
                    .body(UpdatedBalanceResponseDto.class);
        } catch (HttpClientErrorException.BadRequest e) {
            throw new BusinessException(ErrorCode.INSUFFICIENT_FUNDS, "Neuspešan transfer: " + e.getResponseBodyAsString());
        } catch (HttpClientErrorException.UnprocessableEntity e) {
            // Account service vraca 422 za INSUFFICIENT_FUNDS, DAILY_LIMIT_EXCEEDED, MONTHLY_LIMIT_EXCEEDED
            throw new BusinessException(ErrorCode.INSUFFICIENT_FUNDS, "Neuspešan transfer: " + e.getResponseBodyAsString());
        } catch (HttpClientErrorException.Forbidden e) {
            throw new BusinessException(ErrorCode.ACCOUNT_OWNERSHIP_MISMATCH, "Account service: Narušeno vlasništvo nad računom.");
        } catch (Exception e) {
            log.error("Critical error during account transfer: {}", e.getMessage());
            throw new BusinessException(ErrorCode.TRANSFER_NOT_FOUND, "Greška prilikom izvršavanja transfera u Account servisu.");
        }
    }
}
