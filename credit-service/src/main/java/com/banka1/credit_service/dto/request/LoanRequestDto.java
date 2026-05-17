package com.banka1.credit_service.dto.request;

import com.banka1.credit_service.domain.enums.*;
import jakarta.persistence.Column;
import jakarta.persistence.EnumType;
import jakarta.persistence.Enumerated;
import jakarta.validation.constraints.NotBlank;
import jakarta.validation.constraints.NotNull;
import jakarta.validation.constraints.Pattern;
import jakarta.validation.constraints.Positive;
import lombok.AllArgsConstructor;
import lombok.Getter;
import lombok.NoArgsConstructor;
import lombok.Setter;

import java.math.BigDecimal;

/**
 * DTO for client loan requests.
 * Contains all information required for a loan application.
 * All fields are validated according to their specified constraints.
 */
@NoArgsConstructor
@AllArgsConstructor
@Getter
@Setter
public class LoanRequestDto {
    /** The type of loan being requested (e.g., HOME, AUTO, PERSONAL). */
    @NotNull(message = "loanType ne sme biti null")
    private LoanType loanType;

    /** The type of interest rate (FIXED or VARIABLE). */
    @NotNull(message = "interestType ne sme biti null")
    private InterestType interestType;

    /** The requested loan amount (must be positive). */
    @Positive(message = "amount mora biti >0")
    @NotNull(message = "amount ne sme biti null")
    private BigDecimal amount;

    /** The currency code for the loan. */
    @NotNull(message = "currency ne sme biti null")
    private CurrencyCode currency;

    /** The purpose of the loan (cannot be blank). */
    @NotBlank(message = "purpose ne sme biti prazan")
    private String purpose;

    /** The applicant's monthly salary (must be positive). */
    @NotNull(message = "monthlySalary ne sme biti null")
    @Positive(message = "monthlySalary mora biti > 0")
    private BigDecimal monthlySalary;

    /** The applicant's current employment status. */
    @NotNull(message = "employmentStatus ne sme biti null")
    private EmploymentStatus employmentStatus;

    /** The length of current employment period in months (must be positive). */
    @NotNull(message = "currentEmploymentPeriod ne sme biti null")
    private Integer currentEmploymentPeriod;

    /** The requested loan repayment period in months (must be positive). */
    @Positive(message = "repaymentPeriod mora biti pozitivan")
    @NotNull(message = "repaymentPeriod ne sme biti null")
    private Integer repaymentPeriod;

    /** The applicant's contact phone number (cannot be blank). */
    @NotBlank(message = "contactPhone ne sme biti prazan")
    private String contactPhone;

    /** The applicant's bank account number (cannot be blank). */
    @NotBlank(message = "accountNumber ne sme biti prazan")
    @Pattern(regexp = "^\\d{18,19}$", message = "Broj racuna mora imati 18 ili 19 cifara")
    private String accountNumber;
}
