package com.banka1.tradingservice.funds.dto;

import jakarta.validation.constraints.DecimalMin;
import jakarta.validation.constraints.NotBlank;
import jakarta.validation.constraints.NotNull;
import jakarta.validation.constraints.Size;
import lombok.AllArgsConstructor;
import lombok.Data;
import lombok.NoArgsConstructor;

import java.math.BigDecimal;

/** Body za POST /funds — supervizor kreira fond. */
@Data
@NoArgsConstructor
@AllArgsConstructor
public class CreateFundRequest {

    @NotBlank
    @Size(max = 64)
    private String naziv;

    @Size(max = 1024)
    private String opis;

    @NotNull
    @DecimalMin(value = "0.00")
    private BigDecimal minimumContribution;
}
