package com.banka1.banking_service.card_service.rest_client;

import com.banka1.banking_service.card_service.domain.enums.AccountOwnershipType;

/**
 * Internal account-service DTO used to determine account ownership/type
 * for card requests and downstream notifications.
 *
 * @param ownershipType ownership of the linked account
 * @param ownerClientId client ID of the business-account owner
 */
public record AccountNotificationContextDto(
        AccountOwnershipType ownershipType,
        Long ownerClientId
) {

    public boolean isBusinessAccount() {
        return ownershipType == AccountOwnershipType.BUSINESS;
    }
}
