package com.banka1.banking_service.credit_service.dto.request;

import lombok.AllArgsConstructor;
import lombok.Getter;
import lombok.NoArgsConstructor;
import lombok.Setter;

import java.math.BigDecimal;

/**
 * DTO for financial transaction or transfer requests.
 * <p>
 * Used for intra-bank transfers between accounts with support for
 * currency conversion and commission calculation.
 * <p>
 * Validation constraints:
 * <ul>
 *   <li>Both account numbers must be 19 digits</li>
 *   <li>Amounts (fromAmount and toAmount) must be positive</li>
 *   <li>Commission must be >= 0</li>
 * </ul>
 */
@AllArgsConstructor
@NoArgsConstructor
@Getter
@Setter
public class PaymentDto {
    /**
     * The account number from which funds are transferred (19 digits).
     */
    private String fromAccountNumber;

    /**
     * The account number to which funds are transferred (19 digits).
     */
    private String toAccountNumber;

    /**
     * The amount transferred from the source account in its currency.
     * <p>
     * If the accounts are in different currencies, this amount is converted
     * according to toAmount.
     */
    private BigDecimal fromAmount;

    /**
     * The amount received at the destination account after conversion (if applicable).
     * <p>
     * If the accounts are in the same currency, this value equals fromAmount.
     * If the accounts are in different currencies, this value is converted according to the exchange rate.
     */
    private BigDecimal toAmount;

    /**
     * The commission for the transaction. Usually deducted from the source account.
     */
    private BigDecimal commission;

    /**
     * The ID of the client initiating the transfer (optional, for audit logging).
     */

    private Long clientId;


}
