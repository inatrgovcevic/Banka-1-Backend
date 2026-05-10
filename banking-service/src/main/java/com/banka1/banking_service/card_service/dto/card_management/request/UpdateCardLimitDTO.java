package com.banka1.banking_service.card_service.dto.card_management.request;

import jakarta.validation.constraints.DecimalMin;
import jakarta.validation.constraints.NotNull;
import lombok.Getter;
import lombok.NoArgsConstructor;
import lombok.Setter;

import java.math.BigDecimal;

/**
 * Request body for updating the spending limit on an existing card.
 * The new limit must be a non-negative value — zero is allowed to effectively disable spending.
 */
@Getter
@Setter
@NoArgsConstructor
public class UpdateCardLimitDTO {

    @NotNull(message = "Card limit is required.")
    @DecimalMin(value = "0.0", message = "Card limit must be zero or greater.")
    private BigDecimal cardLimit;
}
