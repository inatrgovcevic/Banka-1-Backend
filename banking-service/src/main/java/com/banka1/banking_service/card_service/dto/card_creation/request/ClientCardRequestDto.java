package com.banka1.banking_service.card_service.dto.card_creation.request;

import com.banka1.banking_service.card_service.domain.enums.CardBrand;
import jakarta.validation.constraints.NotNull;
import lombok.Getter;
import lombok.Setter;

import java.math.BigDecimal;

/**
 * Personal-account card request payload.
 */
@Getter
@Setter
public class ClientCardRequestDto {

    private String accountNumber;

    private CardBrand cardBrand;

    private BigDecimal cardLimit;

    @NotNull(message = "verificationId is required.")
    private Long verificationId;
}
