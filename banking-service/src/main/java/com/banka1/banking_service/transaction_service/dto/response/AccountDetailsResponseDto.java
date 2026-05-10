package com.banka1.banking_service.transaction_service.dto.response;

import com.fasterxml.jackson.annotation.JsonProperty;
import lombok.AllArgsConstructor;
import lombok.Getter;
import lombok.NoArgsConstructor;
import lombok.Setter;

/**
 * DTO representing account details.
 * Contains information about a specific account.
 */
@Getter
@Setter
@AllArgsConstructor
@NoArgsConstructor
public class AccountDetailsResponseDto {

    /** Account number associated with the account. */
    @JsonProperty("ownerId")
    private Long vlasnik;
}
