package com.banka1.banking_service.credit_service.service;

import com.banka1.banking_service.credit_service.domain.LoanRequest;
import com.banka1.banking_service.credit_service.domain.enums.LoanType;
import com.banka1.banking_service.credit_service.domain.enums.Status;
import com.banka1.banking_service.credit_service.dto.request.LoanRequestDto;
import com.banka1.banking_service.credit_service.dto.response.LoanInfoResponseDto;
import com.banka1.banking_service.credit_service.dto.response.LoanRequestResponseDto;
import com.banka1.banking_service.credit_service.dto.response.LoanResponseDto;
import org.springframework.data.domain.Page;
import org.springframework.security.oauth2.jwt.Jwt;

/**
 * Service interface for managing loan operations.
 * Defines contract for loan request creation, approval/denial, and loan information retrieval.
 */
public interface LoanService {
    /**
     * Creates a new loan request.
     *
     * @param jwt            the JWT token containing client information
     * @param loanRequestDto the loan request details
     * @return the created loan request response
     */
    LoanRequestResponseDto request(Jwt jwt, LoanRequestDto loanRequestDto);

    /**
     * Confirms (approves or declines) a pending loan request.
     *
     * @param jwt    the JWT token containing employee information
     * @param id     the loan request ID
     * @param status the status to set (APPROVED or DECLINED)
     * @return confirmation message
     */
    String confirmation(Jwt jwt, Long id, Status status);

    /**
     * Retrieves paginated list of loans for the authenticated client.
     *
     * @param jwt  the JWT token containing client information
     * @param page the page number
     * @param size the page size
     * @return page of loan response DTOs
     */
    Page<LoanResponseDto> find(Jwt jwt, int page, int size);

    /**
     * Retrieves detailed information about a specific loan.
     *
     * @param jwt the JWT token containing authentication information
     * @param id  the loan ID
     * @return detailed loan information
     */
    LoanInfoResponseDto info(Jwt jwt, Long id);

    /**
     * Retrieves all pending loan requests with optional filtering.
     *
     * @param jwt          the JWT token containing employee information
     * @param vrstaKredita optional loan type filter
     * @param brojRacuna   optional account number filter
     * @param page         the page number
     * @param size         the page size
     * @return page of loan requests
     */
    Page<LoanRequest> findAllLoanRequest(Jwt jwt, LoanType vrstaKredita, String brojRacuna, int page, int size);

    /**
     * Retrieves all loans with optional filtering.
     *
     * @param jwt          the JWT token containing employee information
     * @param vrstaKredita optional loan type filter
     * @param brojRacuna   optional account number filter
     * @param status       optional loan status filter
     * @param page         the page number
     * @param size         the page size
     * @return page of loan response DTOs
     */
    Page<LoanResponseDto> findAllLoans(Jwt jwt, LoanType vrstaKredita, String brojRacuna, Status status, int page, int size);
}

