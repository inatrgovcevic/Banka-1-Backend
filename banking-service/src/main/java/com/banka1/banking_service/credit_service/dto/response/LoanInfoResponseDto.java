package com.banka1.banking_service.credit_service.dto.response;

import com.banka1.banking_service.credit_service.dto.response.InstallmentResponseDto;
import com.banka1.banking_service.credit_service.dto.response.LoanResponseDto;
import lombok.AllArgsConstructor;
import lombok.Getter;
import lombok.NoArgsConstructor;
import lombok.Setter;

import java.util.List;

@AllArgsConstructor
@NoArgsConstructor
@Getter
@Setter
public class LoanInfoResponseDto {
    private LoanResponseDto loan;
    private List<InstallmentResponseDto> installments;
}
