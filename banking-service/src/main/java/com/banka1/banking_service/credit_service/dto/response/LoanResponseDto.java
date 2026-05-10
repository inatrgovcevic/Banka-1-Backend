package com.banka1.banking_service.credit_service.dto.response;

import com.banka1.banking_service.credit_service.domain.Loan;
import com.banka1.banking_service.credit_service.domain.enums.InterestType;
import com.banka1.banking_service.credit_service.domain.enums.LoanType;
import com.banka1.banking_service.credit_service.domain.enums.Status;
import com.fasterxml.jackson.annotation.JsonInclude;
import lombok.AllArgsConstructor;
import lombok.Getter;
import lombok.NoArgsConstructor;
import lombok.Setter;

import java.math.BigDecimal;
import java.time.LocalDate;

/**
 * DTO for loan response containing comprehensive loan information.
 * Used for both basic and detailed loan representations.
 * Null values are excluded from JSON serialization.
 */
@AllArgsConstructor
@NoArgsConstructor
@Getter
@Setter
@JsonInclude(JsonInclude.Include.NON_NULL)
public class LoanResponseDto {
    /** Unique loan number/ID. */
    private Long loanNumber;

    /** Type of the loan (HOME, AUTO, PERSONAL, etc.). */
    private LoanType loanType;

    /** Account number associated with the loan. */
    private String accountNumber;

    /** Principal amount of the loan. */
    private BigDecimal amount;

    /** Repayment method/period in months. */
    private Integer repaymentMethod;

    /** Nominal annual interest rate. */
    private BigDecimal nominalInterestRate;

    /** Effective annual interest rate (includes fees). */
    private BigDecimal effectiveInterestRate;

    /** Type of interest rate (FIXED or VARIABLE). */
    private InterestType interestType;

    /** Date when the loan agreement was made. */
    private LocalDate agreementDate;

    /** Date when the loan matures/is due. */
    private LocalDate maturityDate;

    /** Monthly installment amount. */
    private BigDecimal installmentAmount;

    /** Date of the next scheduled installment. */
    private LocalDate nextInstallmentDate;

    /** Remaining debt amount on the loan. */
    private BigDecimal remainingDebt;

    /** Current status of the loan (ACTIVE, APPROVED, DECLINED, etc.). */
    private Status status;

    /**
     * Constructs a minimal LoanResponseDto with basic information.
     *
     * @param loanNumber the loan ID
     * @param loanType the type of loan
     * @param amount the loan principal amount
     * @param status the loan status
     */
    public LoanResponseDto(Long loanNumber, LoanType loanType, BigDecimal amount, Status status) {
        this.loanNumber = loanNumber;
        this.loanType = loanType;
        this.amount = amount;
        this.status = status;
    }

    /**
     * Constructs a LoanResponseDto from a Loan entity, populating all fields.
     *
     * @param loan the Loan entity to convert
     */
    public LoanResponseDto(Loan loan)
    {
        this.loanNumber=loan.getId();
        this.loanType=loan.getLoanType();
        this.accountNumber=loan.getAccountNumber();
        this.amount=loan.getAmount();
        this.repaymentMethod=loan.getRepaymentPeriod();
        this.nominalInterestRate=loan.getNominalInterestRate();
        this.effectiveInterestRate=loan.getEffectiveInterestRate();
        this.interestType=loan.getInterestType();
        this.agreementDate=loan.getAgreementDate();
        this.maturityDate=loan.getMaturityDate();
        this.installmentAmount=loan.getInstallmentAmount();
        this.nextInstallmentDate=loan.getNextInstallmentDate();
        this.remainingDebt=loan.getRemainingDebt();
        this.status=loan.getStatus();
    }

}
