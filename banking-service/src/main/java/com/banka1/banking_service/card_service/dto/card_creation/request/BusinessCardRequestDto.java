package com.banka1.banking_service.card_service.dto.card_creation.request;

import com.banka1.banking_service.card_service.domain.enums.CardBrand;
import com.banka1.banking_service.card_service.domain.enums.CardRequestRecipientType;
import jakarta.validation.Valid;
import jakarta.validation.constraints.NotNull;
import lombok.Getter;
import lombok.Setter;

import java.math.BigDecimal;

/**
 * Business-account card request payload.
 */
@Getter
@Setter
public class BusinessCardRequestDto {

    private String accountNumber;

    private CardRequestRecipientType recipientType;

    private Long authorizedPersonId;

    @Valid
    private AuthorizedPersonRequestDto authorizedPerson;

    private CardBrand cardBrand;

    private BigDecimal cardLimit;

    @NotNull(message = "verificationId is required.")
    private Long verificationId;
}
