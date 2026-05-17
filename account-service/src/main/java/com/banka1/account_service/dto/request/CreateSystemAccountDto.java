package com.banka1.account_service.dto.request;

import com.banka1.account_service.domain.enums.AccountConcrete;
import com.banka1.account_service.domain.enums.CurrencyCode;
import jakarta.validation.constraints.NotBlank;
import jakarta.validation.constraints.NotNull;
import jakarta.validation.constraints.Pattern;
import lombok.AllArgsConstructor;
import lombok.Data;
import lombok.NoArgsConstructor;

import java.math.BigDecimal;

/**
 * Payload za POST /internal/accounts/system (PR_14 C14.8).
 *
 * <p>Koristi se da trading-service moze pri kreiranju investicionog fonda da
 * trazi pravi Account red u account-service-u (do PR_14 fond je imao samo
 * "fictional" account_number string, pa SAGA invest/redeem nisu mogli da
 * skidaju/dodaju novac ka tom racunu).
 */
@Data
@NoArgsConstructor
@AllArgsConstructor
public class CreateSystemAccountDto {

    @NotBlank
    @Pattern(regexp = "^\\d{16}(\\d{3})?$", message = "Account number mora imati 16 ili 19 cifara")
    private String accountNumber;

    @NotNull
    private Long ownerId;

    @NotNull
    private CurrencyCode currencyCode;

    @NotNull
    private AccountConcrete accountConcrete;

    @NotBlank
    private String displayName;

    private BigDecimal initialBalance = BigDecimal.ZERO;
}
