package com.banka1.banking_service.credit_service.integration;

import com.banka1.banking_service.credit_service.domain.Installment;
import com.banka1.banking_service.credit_service.domain.Loan;
import com.banka1.banking_service.credit_service.domain.LoanRequest;
import com.banka1.banking_service.credit_service.domain.enums.*;
import com.banka1.banking_service.credit_service.dto.request.LoanRequestDto;
import com.banka1.banking_service.credit_service.rabbitMQ.RabbitClient;
import com.banka1.banking_service.credit_service.repository.InstallmentRepository;
import com.banka1.banking_service.credit_service.repository.LoanRepository;
import com.banka1.banking_service.credit_service.repository.LoanRequestRepository;
import com.banka1.banking_service.account_service.dto.response.InternalAccountDetailsDto;
import com.banka1.banking_service.account_service.service.AccountService;
import com.banka1.banking_service.credit_service.rest_client.ClientService;
import com.banka1.banking_service.credit_service.rest_client.ExchangeService;
import com.banka1.banking_service.credit_service.domain.enums.CurrencyCode;
import com.fasterxml.jackson.databind.ObjectMapper;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.test.context.SpringBootTest;
import org.springframework.boot.webmvc.test.autoconfigure.AutoConfigureMockMvc;
import org.springframework.security.core.authority.SimpleGrantedAuthority;
import org.springframework.security.test.web.servlet.request.SecurityMockMvcRequestPostProcessors;
import org.springframework.test.context.ActiveProfiles;
import org.springframework.test.context.bean.override.mockito.MockitoBean;
import org.springframework.test.web.servlet.MockMvc;

import java.math.BigDecimal;
import java.time.LocalDate;

import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.Mockito.doNothing;
import static org.mockito.Mockito.when;
import static org.springframework.http.MediaType.APPLICATION_JSON;
import static org.springframework.test.web.servlet.request.MockMvcRequestBuilders.*;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.jsonPath;
import static org.springframework.test.web.servlet.result.MockMvcResultMatchers.status;

@SpringBootTest
@AutoConfigureMockMvc
@ActiveProfiles("test")
class LoanControllerIntegrationTest {

    @Autowired
    private MockMvc mockMvc;

    private final ObjectMapper objectMapper = new ObjectMapper().findAndRegisterModules();

    @Autowired
    private LoanRequestRepository loanRequestRepository;

    @Autowired
    private LoanRepository loanRepository;

    @Autowired
    private InstallmentRepository installmentRepository;

    @MockitoBean
    private AccountService accountService;

    @MockitoBean
    private ExchangeService exchangeService;

    @MockitoBean
    private ClientService clientService;

    @MockitoBean
    private RabbitClient rabbitClient;

    @BeforeEach
    void setUp() {
        installmentRepository.deleteAll();
        loanRepository.deleteAll();
        loanRequestRepository.deleteAll();
        doNothing().when(rabbitClient).sendEmailNotification(any());
        doNothing().when(clientService).addMarginPermission(any());
    }

    @Test
    void requestEndpointPersistsLoanRequestForAuthenticatedOwner() throws Exception {
        LoanRequestDto request = validRequest();
        when(accountService.getAccountDetails("1234567890123456789"))
                .thenReturn(accountDetails("1234567890123456789", 77L, "RSD"));

        mockMvc.perform(post("/api/loans/requests")
                        .with(SecurityMockMvcRequestPostProcessors.jwt()
                                .jwt(jwt -> jwt.claim("id", 77L).claim("roles", "CLIENT_BASIC"))
                                .authorities(new SimpleGrantedAuthority("ROLE_CLIENT_BASIC")))
                        .with(SecurityMockMvcRequestPostProcessors.csrf())
                        .contentType(APPLICATION_JSON)
                        .content(objectMapper.writeValueAsString(request)))
                .andExpect(status().isCreated())
                .andExpect(jsonPath("$.id").isNumber())
                .andExpect(jsonPath("$.createdAt").isNotEmpty());

        LoanRequest persisted = loanRequestRepository.findAll().getFirst();
        assertThat(persisted.getClientId()).isEqualTo(77L);
        assertThat(persisted.getStatus()).isEqualTo(Status.PENDING);
        assertThat(persisted.getAccountNumber()).isEqualTo("1234567890123456789");
    }

    @Test
    void requestEndpointReturnsBadRequestForBusinessRuleViolation() throws Exception {
        LoanRequestDto request = validRequest();
        request.setRepaymentPeriod(11);

        mockMvc.perform(post("/api/loans/requests")
                        .with(SecurityMockMvcRequestPostProcessors.jwt()
                                .jwt(jwt -> jwt.claim("id", 77L).claim("roles", "CLIENT_BASIC"))
                                .authorities(new SimpleGrantedAuthority("ROLE_CLIENT_BASIC")))
                        .with(SecurityMockMvcRequestPostProcessors.csrf())
                        .contentType(APPLICATION_JSON)
                        .content(objectMapper.writeValueAsString(request)))
                .andExpect(status().isBadRequest())
                .andExpect(jsonPath("$.errorCode").value("ERR_VALIDATION"))
                .andExpect(jsonPath("$.errorTitle").value("Neispravni argumenti"));
    }

    @Test
    void requestEndpointReturnsValidationErrorsForMalformedPayload() throws Exception {
        LoanRequestDto request = validRequest();
        request.setPurpose("");
        request.setAmount(BigDecimal.ZERO);

        mockMvc.perform(post("/api/loans/requests")
                        .with(SecurityMockMvcRequestPostProcessors.jwt()
                                .jwt(jwt -> jwt.claim("id", 77L).claim("roles", "CLIENT_BASIC"))
                                .authorities(new SimpleGrantedAuthority("ROLE_CLIENT_BASIC")))
                        .with(SecurityMockMvcRequestPostProcessors.csrf())
                        .contentType(APPLICATION_JSON)
                        .content(objectMapper.writeValueAsString(request)))
                .andExpect(status().isBadRequest())
                .andExpect(jsonPath("$.errorCode").value("ERR_VALIDATION"))
                .andExpect(jsonPath("$.validationErrors.amount").value("amount mora biti >0"))
                .andExpect(jsonPath("$.validationErrors.purpose").value("purpose ne sme biti prazan"));
    }

    @Test
    void requestEndpointRejectsEmployeeRole() throws Exception {
        LoanRequestDto request = validRequest();

        mockMvc.perform(post("/api/loans/requests")
                        .with(SecurityMockMvcRequestPostProcessors.jwt()
                                .jwt(jwt -> jwt.claim("id", 501L).claim("roles", "BASIC"))
                                .authorities(new SimpleGrantedAuthority("ROLE_BASIC")))
                        .with(SecurityMockMvcRequestPostProcessors.csrf())
                        .contentType(APPLICATION_JSON)
                        .content(objectMapper.writeValueAsString(request)))
                .andExpect(status().isForbidden());
    }

    @Test
    void approveEndpointCreatesLoanAndFirstInstallment() throws Exception {
        LoanRequest loanRequest = loanRequestRepository.save(pendingRequest());
        doNothing().when(accountService).transactionFromBank(any());

        mockMvc.perform(put("/api/loans/requests/{id}/approve", loanRequest.getId())
                        .with(SecurityMockMvcRequestPostProcessors.jwt()
                                .jwt(jwt -> jwt.claim("id", 501L).claim("roles", "BASIC"))
                                .authorities(new SimpleGrantedAuthority("ROLE_BASIC")))
                        .with(SecurityMockMvcRequestPostProcessors.csrf()))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$").value("ODOBREN ZAHTEV"));

        LoanRequest updatedRequest = loanRequestRepository.findById(loanRequest.getId()).orElseThrow();
        assertThat(updatedRequest.getStatus()).isEqualTo(Status.APPROVED);

        Loan createdLoan = loanRepository.findAll().getFirst();
        assertThat(createdLoan.getStatus()).isEqualTo(Status.ACTIVE);
        assertThat(createdLoan.getClientId()).isEqualTo(loanRequest.getClientId());
        assertThat(createdLoan.getAccountNumber()).isEqualTo(loanRequest.getAccountNumber());

        Installment createdInstallment = installmentRepository.findAll().getFirst();
        assertThat(createdInstallment.getLoan().getId()).isEqualTo(createdLoan.getId());
        assertThat(createdInstallment.getPaymentStatus()).isEqualTo(PaymentStatus.UNPAID);
    }

    @Test
    void approveEndpointRejectsClientRole() throws Exception {
        LoanRequest loanRequest = loanRequestRepository.save(pendingRequest());

        mockMvc.perform(put("/api/loans/requests/{id}/approve", loanRequest.getId())
                        .with(SecurityMockMvcRequestPostProcessors.jwt()
                                .jwt(jwt -> jwt.claim("id", 77L).claim("roles", "CLIENT_BASIC"))
                                .authorities(new SimpleGrantedAuthority("ROLE_CLIENT_BASIC")))
                        .with(SecurityMockMvcRequestPostProcessors.csrf()))
                .andExpect(status().isForbidden());
    }

    @Test
    void declineEndpointUpdatesRequestStatusAndSkipsLoanCreation() throws Exception {
        LoanRequest loanRequest = loanRequestRepository.save(pendingRequest());

        mockMvc.perform(put("/api/loans/requests/{id}/decline", loanRequest.getId())
                        .with(SecurityMockMvcRequestPostProcessors.jwt()
                                .jwt(jwt -> jwt.claim("id", 501L).claim("roles", "BASIC"))
                                .authorities(new SimpleGrantedAuthority("ROLE_BASIC")))
                        .with(SecurityMockMvcRequestPostProcessors.csrf()))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$").value("ODBIJEN ZAHTEV"));

        LoanRequest updatedRequest = loanRequestRepository.findById(loanRequest.getId()).orElseThrow();
        assertThat(updatedRequest.getStatus()).isEqualTo(Status.DECLINED);
        assertThat(loanRepository.findAll()).isEmpty();
        assertThat(installmentRepository.findAll()).isEmpty();
    }

    @Test
    void infoEndpointReturnsLoanDetailsToOwner() throws Exception {
        Loan loan = loanRepository.save(activeLoan());
        installmentRepository.save(new Installment(
                loan,
                new BigDecimal("22100.00"),
                new BigDecimal("0.0060"),
                CurrencyCode.RSD,
                LocalDate.now().plusMonths(1),
                null,
                PaymentStatus.UNPAID
        ));

        mockMvc.perform(get("/api/loans/{loanNumber}", loan.getId())
                        .with(SecurityMockMvcRequestPostProcessors.jwt()
                                .jwt(jwt -> jwt.claim("id", 77L).claim("roles", "CLIENT_BASIC"))
                                .authorities(new SimpleGrantedAuthority("ROLE_CLIENT_BASIC"))))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.loan.loanNumber").value(loan.getId()))
                .andExpect(jsonPath("$.loan.status").value("ACTIVE"))
                .andExpect(jsonPath("$.installments.length()").value(1))
                .andExpect(jsonPath("$.installments[0].paymentStatus").value("UNPAID"));
    }

    @Test
    void infoEndpointReturnsBadRequestForDifferentClient() throws Exception {
        Loan loan = loanRepository.save(activeLoan());

        mockMvc.perform(get("/api/loans/{loanNumber}", loan.getId())
                        .with(SecurityMockMvcRequestPostProcessors.jwt()
                                .jwt(jwt -> jwt.claim("id", 88L).claim("roles", "CLIENT_BASIC"))
                                .authorities(new SimpleGrantedAuthority("ROLE_CLIENT_BASIC"))))
                .andExpect(status().isBadRequest())
                .andExpect(jsonPath("$.errorCode").value("ERR_VALIDATION"));
    }

    @Test
    void clientEndpointReturnsOnlyAuthenticatedClientLoans() throws Exception {
        Loan ownLoan = activeLoan();
        ownLoan.setAmount(new BigDecimal("700000.00"));
        Loan otherLoan = activeLoan();
        otherLoan.setAccountNumber("ACC-999");
        otherLoan.setClientId(999L);
        otherLoan.setAmount(new BigDecimal("900000.00"));
        loanRepository.save(ownLoan);
        loanRepository.save(otherLoan);

        mockMvc.perform(get("/api/loans/client")
                        .param("page", "0")
                        .param("size", "10")
                        .with(SecurityMockMvcRequestPostProcessors.jwt()
                                .jwt(jwt -> jwt.claim("id", 77L).claim("roles", "CLIENT_BASIC"))
                                .authorities(new SimpleGrantedAuthority("ROLE_CLIENT_BASIC"))))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.content.length()").value(1))
                .andExpect(jsonPath("$.content[0].amount").value(700000.00));
    }

    @Test
    void findLoanRequestsEndpointSupportsFiltersForEmployees() throws Exception {
        LoanRequest matching = pendingRequest();
        LoanRequest other = pendingRequest();
        other.setAccountNumber("ACC-XYZ");
        other.setLoanType(LoanType.STUDENTSKI);
        loanRequestRepository.save(matching);
        loanRequestRepository.save(other);

        mockMvc.perform(get("/api/loans/requests")
                        .param("vrstaKredita", "AUTO")
                        .param("brojRacuna", "ACC-001")
                        .with(SecurityMockMvcRequestPostProcessors.jwt()
                                .jwt(jwt -> jwt.claim("id", 501L).claim("roles", "BASIC"))
                                .authorities(new SimpleGrantedAuthority("ROLE_BASIC"))))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.content.length()").value(1))
                .andExpect(jsonPath("$.content[0].accountNumber").value("ACC-001"))
                .andExpect(jsonPath("$.content[0].loanType").value("AUTO"));
    }

    @Test
    void findLoanRequestsEndpointRejectsInvalidLoanTypeFilter() throws Exception {
        mockMvc.perform(get("/api/loans/requests")
                        .param("vrstaKredita", "NOT_A_TYPE")
                        .with(SecurityMockMvcRequestPostProcessors.jwt()
                                .jwt(jwt -> jwt.claim("id", 501L).claim("roles", "BASIC"))
                                .authorities(new SimpleGrantedAuthority("ROLE_BASIC"))))
                .andExpect(status().isBadRequest())
                .andExpect(jsonPath("$.errorCode").value("ERR_VALIDATION"))
                .andExpect(jsonPath("$.errorDesc").value("Los loanType"));
    }

    @Test
    void findAllLoansEndpointSupportsFiltersForEmployees() throws Exception {
        Loan matching = activeLoan();
        Loan other = activeLoan();
        other.setLoanType(LoanType.STUDENTSKI);
        other.setAccountNumber("ACC-XYZ");
        other.setStatus(Status.OVERDUE);
        loanRepository.save(matching);
        loanRepository.save(other);

        mockMvc.perform(get("/api/loans/all")
                        .param("vrstaKredita", "AUTO")
                        .param("brojRacuna", "ACC-001")
                        .param("loanStatus", "ACTIVE")
                        .with(SecurityMockMvcRequestPostProcessors.jwt()
                                .jwt(jwt -> jwt.claim("id", 501L).claim("roles", "BASIC"))
                                .authorities(new SimpleGrantedAuthority("ROLE_BASIC"))))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.content.length()").value(1))
                .andExpect(jsonPath("$.content[0].loanType").value("AUTO"))
                .andExpect(jsonPath("$.content[0].status").value("ACTIVE"));
    }

    @Test
    void findAllLoansEndpointRejectsInvalidStatusFilter() throws Exception {
        mockMvc.perform(get("/api/loans/all")
                        .param("loanStatus", "NOT_A_STATUS")
                        .with(SecurityMockMvcRequestPostProcessors.jwt()
                                .jwt(jwt -> jwt.claim("id", 501L).claim("roles", "BASIC"))
                                .authorities(new SimpleGrantedAuthority("ROLE_BASIC"))))
                .andExpect(status().isBadRequest())
                .andExpect(jsonPath("$.errorCode").value("ERR_VALIDATION"))
                .andExpect(jsonPath("$.errorDesc").value("Los loanStatus"));
    }

    @Test
    void findAllLoansEndpointReturnsLoansForAdminRoleThroughHierarchy() throws Exception {
        Loan loan = loanRepository.save(activeLoan());

        mockMvc.perform(get("/api/loans/all")
                        .with(SecurityMockMvcRequestPostProcessors.jwt()
                                .jwt(jwt -> jwt.claim("id", 999L).claim("roles", "ADMIN"))
                                .authorities(new SimpleGrantedAuthority("ROLE_ADMIN"))))
                .andExpect(status().isOk())
                .andExpect(jsonPath("$.content.length()").value(1))
                .andExpect(jsonPath("$.content[0].loanNumber").value(loan.getId()))
                .andExpect(jsonPath("$.content[0].loanType").value("AUTO"))
                .andExpect(jsonPath("$.content[0].accountNumber").value("ACC-001"))
                .andExpect(jsonPath("$.content[0].status").value("ACTIVE"));
    }

    private LoanRequestDto validRequest() {
        return new LoanRequestDto(
                LoanType.AUTO,
                InterestType.FIXED,
                new BigDecimal("500000.00"),
                CurrencyCode.RSD,
                "Kupovina automobila",
                new BigDecimal("150000.00"),
                EmploymentStatus.PERMANENT,
                48,
                24,
                "+38160111222",
                "1234567890123456789"
        );
    }

    private LoanRequest pendingRequest() {
        return new LoanRequest(
                LoanType.AUTO,
                InterestType.FIXED,
                new BigDecimal("500000.00"),
                CurrencyCode.RSD,
                "Kupovina automobila",
                new BigDecimal("150000.00"),
                EmploymentStatus.PERMANENT,
                48,
                24,
                "+38160111222",
                "ACC-001",
                77L,
                Status.PENDING,
                "pera@test.com",
                "pera"
        );
    }

    private Loan activeLoan() {
        Loan loan = new Loan();
        loan.setLoanType(LoanType.AUTO);
        loan.setAccountNumber("ACC-001");
        loan.setAmount(new BigDecimal("500000.00"));
        loan.setRepaymentPeriod(24);
        loan.setNominalInterestRate(new BigDecimal("0.0060"));
        loan.setEffectiveInterestRate(new BigDecimal("0.0060"));
        loan.setInterestType(InterestType.FIXED);
        loan.setAgreementDate(LocalDate.now().minusMonths(1));
        loan.setMaturityDate(LocalDate.now().plusMonths(23));
        loan.setInstallmentAmount(new BigDecimal("22100.00"));
        loan.setNextInstallmentDate(LocalDate.now().plusMonths(1));
        loan.setRemainingDebt(new BigDecimal("480000.00"));
        loan.setCurrency(CurrencyCode.RSD);
        loan.setStatus(Status.ACTIVE);
        loan.setUserEmail("pera@test.com");
        loan.setUsername("pera");
        loan.setClientId(77L);
        loan.setInstallmentCount(1);
        return loan;
    }

    private InternalAccountDetailsDto accountDetails(String accountNumber, Long ownerId, String currency) {
        return new InternalAccountDetailsDto(
                null,
                accountNumber,
                ownerId,
                currency,
                BigDecimal.TEN,
                "ACTIVE",
                "CURRENT",
                "pera@test.com",
                "pera"
        );
    }
}
