package com.banka1.banking_service.card_service.dto.card_creation.request;

import lombok.Getter;
import lombok.Setter;

import java.time.LocalDate;

/**
 * DTO that we receive from account-service, and use for `Automatic` card creation.
 */
@Getter
@Setter
public class AutoCardCreationRequestDto {
    private Long clientId;
    private String accountNumber;
    private String accountCurrency;
    private String accountCategory;
    private String accountType;
    private String accountSubtype;
    private String ownerFirstName;
    private String ownerLastName;
    private String ownerEmail;
    private String ownerUsername;
    private LocalDate accountExpirationDate;
}
