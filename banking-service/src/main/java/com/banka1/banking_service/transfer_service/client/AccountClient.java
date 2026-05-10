package com.banka1.banking_service.transfer_service.client;

import com.banka1.banking_service.transfer_service.dto.client.AccountDto;
import com.banka1.banking_service.account_service.dto.request.PaymentDto;
import com.banka1.banking_service.transfer_service.dto.client.UpdatedBalanceResponseDto;

/**
 * Interfejs za komunikaciju sa servisom za upravljanje računima (Account Service).
 */
public interface AccountClient {
    /**
     * Dobavlja detaljne informacije o bankovnom računu na osnovu broja računa.
     * @param accountNumber jedinstveni broj računa
     * @return DTO sa podacima o vlasniku, valuti i balansu
     */
    AccountDto getAccountDetails(String accountNumber);
    /**
     * Izvršava atomsku transakciju transfera sredstava između dva računa u Account servisu.
     * @param paymentDto podaci o pošiljaocu, primaocu, iznosima i proviziji
     * @return DTO sa ažuriranim stanjima na oba računa
     */
    UpdatedBalanceResponseDto executeTransfer(PaymentDto paymentDto);
}
