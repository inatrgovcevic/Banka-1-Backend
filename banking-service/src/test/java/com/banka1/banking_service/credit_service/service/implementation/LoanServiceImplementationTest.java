package com.banka1.banking_service.credit_service.service.implementation;

import com.banka1.banking_service.credit_service.domain.Installment;
import com.banka1.banking_service.credit_service.domain.Loan;
import com.banka1.banking_service.credit_service.domain.LoanRequest;
import com.banka1.banking_service.credit_service.domain.enums.*;
import com.banka1.banking_service.account_service.dto.request.BankPaymentDto;
import com.banka1.banking_service.account_service.dto.response.InternalAccountDetailsDto;
import com.banka1.banking_service.credit_service.dto.request.LoanRequestDto;
import com.banka1.banking_service.credit_service.dto.response.LoanInfoResponseDto;
import com.banka1.banking_service.credit_service.dto.response.LoanRequestResponseDto;
import com.banka1.banking_service.credit_service.dto.response.LoanResponseDto;
import com.banka1.banking_service.credit_service.rabbitMQ.EmailDto;
import com.banka1.banking_service.credit_service.rabbitMQ.EmailType;
import com.banka1.banking_service.credit_service.rabbitMQ.RabbitClient;
import com.banka1.banking_service.credit_service.repository.InstallmentRepository;
import com.banka1.banking_service.credit_service.repository.LoanRepository;
import com.banka1.banking_service.credit_service.repository.LoanRequestRepository;
import com.banka1.banking_service.credit_service.rest_client.ClientService;
import com.banka1.banking_service.credit_service.rest_client.ExchangeService;
import com.banka1.banking_service.account_service.service.AccountService;
import com.banka1.banking_service.credit_service.domain.enums.CurrencyCode;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.ArgumentCaptor;
import org.mockito.InjectMocks;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;
import org.springframework.data.domain.PageImpl;
import org.springframework.data.domain.PageRequest;
import org.springframework.http.HttpStatus;
import org.springframework.security.oauth2.jwt.Jwt;
import org.springframework.test.util.ReflectionTestUtils;
import org.springframework.transaction.support.TransactionSynchronization;
import org.springframework.transaction.support.TransactionSynchronizationManager;
import org.springframework.web.client.HttpClientErrorException;

import java.math.BigDecimal;
import java.time.LocalDate;
import java.time.LocalDateTime;
import java.util.List;
import java.util.Map;
import java.util.Optional;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.eq;
import static org.mockito.Mockito.*;

@ExtendWith(MockitoExtension.class)
class LoanServiceImplementationTest {

    @Mock
    private AccountService accountService;
    @Mock
    private ExchangeService exchangeService;
    @Mock
    private LoanRequestRepository loanRequestRepository;
    @Mock
    private InstallmentRepository installmentRepository;
    @Mock
    private LoanRepository loanRepository;
    @Mock
    private RabbitClient rabbitClient;
    @Mock
    private ClientService clientService;

    @InjectMocks
    private LoanServiceImplementation service;

    @BeforeEach
    void setUp() {
        ReflectionTestUtils.setField(service, "appPropertiesId", "id");
        ReflectionTestUtils.setField(service, "roles", "roles");
        service.setReferenceRate(new BigDecimal("0.0010"));
    }

    @AfterEach
    void tearDown() {
        if (TransactionSynchronizationManager.isSynchronizationActive()) {
            TransactionSynchronizationManager.clearSynchronization();
        }
    }

    @Test
    void requestPersistsLoanRequestForAccountOwner() {
        LoanRequestDto dto = validRequestDto();
        InternalAccountDetailsDto account = accountDetails(77L, CurrencyCode.RSD, "pera@test.com", "pera");
        LoanRequest persisted = new LoanRequest(
                dto.getLoanType(), dto.getInterestType(), dto.getAmount(), dto.getCurrency(), dto.getPurpose(),
                dto.getMonthlySalary(), dto.getEmploymentStatus(), dto.getCurrentEmploymentPeriod(),
                dto.getRepaymentPeriod(), dto.getContactPhone(), dto.getAccountNumber(), 77L,
                Status.PENDING, "pera@test.com", "pera"
        );
        persisted.setId(10L);
        persisted.setCreatedAt(LocalDateTime.of(2026, 4, 9, 20, 0));

        when(accountService.getAccountDetails(dto.getAccountNumber())).thenReturn(account);
        when(loanRequestRepository.save(any(LoanRequest.class))).thenReturn(persisted);

        LoanRequestResponseDto response = service.request(jwt(77L, "CLIENT_BASIC"), dto);

        assertThat(response.getId()).isEqualTo(10L);
        assertThat(response.getCreatedAt()).isEqualTo(persisted.getCreatedAt());

        ArgumentCaptor<LoanRequest> captor = ArgumentCaptor.forClass(LoanRequest.class);
        verify(loanRequestRepository).save(captor.capture());
        LoanRequest saved = captor.getValue();
        assertThat(saved.getStatus()).isEqualTo(Status.PENDING);
        assertThat(saved.getClientId()).isEqualTo(77L);
        assertThat(saved.getUserEmail()).isEqualTo("pera@test.com");
        assertThat(saved.getUsername()).isEqualTo("pera");
    }

    @Test
    void requestRejectsInvalidRepaymentPeriodForHousingLoan() {
        LoanRequestDto dto = validRequestDto();
        dto.setLoanType(LoanType.STAMBENI);
        dto.setRepaymentPeriod(61);

        assertThatThrownBy(() -> service.request(jwt(77L, "CLIENT_BASIC"), dto))
                .isInstanceOf(IllegalArgumentException.class)
                .hasMessageContaining("Nevalidan repaymentPeriod");

        verify(loanRequestRepository, never()).save(any());
    }

    @Test
    void requestRejectsWhenClientDoesNotOwnAccount() {
        LoanRequestDto dto = validRequestDto();
        when(accountService.getAccountDetails(dto.getAccountNumber()))
                .thenReturn(accountDetails(88L, CurrencyCode.RSD, "pera@test.com", "pera"));

        assertThatThrownBy(() -> service.request(jwt(77L, "CLIENT_BASIC"), dto))
                .isInstanceOf(IllegalArgumentException.class)
                .hasMessageContaining("Nisi vlasnik racuna");
    }

    @Test
    void requestRejectsWhenAccountCurrencyDoesNotMatchLoanCurrency() {
        LoanRequestDto dto = validRequestDto();
        when(accountService.getAccountDetails(dto.getAccountNumber()))
                .thenReturn(accountDetails(77L, CurrencyCode.EUR, "pera@test.com", "pera"));

        assertThatThrownBy(() -> service.request(jwt(77L, "CLIENT_BASIC"), dto))
                .isInstanceOf(IllegalArgumentException.class)
                .hasMessageContaining("Valuta racuna ne odgovara valuti kredita");
    }

    @Test
    void requestRejectsWhenAccountDoesNotExist() {
        LoanRequestDto dto = validRequestDto();
        when(accountService.getAccountDetails(dto.getAccountNumber())).thenReturn(null);

        assertThatThrownBy(() -> service.request(jwt(77L, "CLIENT_BASIC"), dto))
                .isInstanceOf(IllegalArgumentException.class)
                .hasMessageContaining("Ne postoji racun");

        verify(loanRequestRepository, never()).save(any());
    }

    @Test
    void requestRejectsInvalidRepaymentPeriodForNonHousingLoan() {
        LoanRequestDto dto = validRequestDto();
        dto.setRepaymentPeriod(13);

        assertThatThrownBy(() -> service.request(jwt(77L, "CLIENT_BASIC"), dto))
                .isInstanceOf(IllegalArgumentException.class)
                .hasMessageContaining("Nevalidan repaymentPeriod");

        verify(loanRequestRepository, never()).save(any());
    }

    @Test
    void confirmationApproveCreatesLoanInstallmentAndApprovalEmail() {
        LoanRequest request = persistedRequest(Status.PENDING);
        when(loanRequestRepository.updateStatus(15L, Status.APPROVED)).thenReturn(1);
        when(loanRequestRepository.findById(15L)).thenReturn(Optional.of(request));
        doNothing().when(accountService).transactionFromBank(any(BankPaymentDto.class));
        when(loanRepository.save(any(Loan.class))).thenAnswer(invocation -> {
            Loan loan = invocation.getArgument(0);
            loan.setId(100L);
            return loan;
        });

        TransactionSynchronizationManager.initSynchronization();

        String result = service.confirmation(jwt(999L, "BASIC"), 15L, Status.APPROVED);

        assertThat(result).isEqualTo("ODOBREN ZAHTEV");

        ArgumentCaptor<Loan> loanCaptor = ArgumentCaptor.forClass(Loan.class);
        verify(loanRepository).save(loanCaptor.capture());
        Loan createdLoan = loanCaptor.getValue();
        assertThat(createdLoan.getStatus()).isEqualTo(Status.ACTIVE);
        assertThat(createdLoan.getAccountNumber()).isEqualTo(request.getAccountNumber());
        assertThat(createdLoan.getClientId()).isEqualTo(request.getClientId());
        assertThat(createdLoan.getNextInstallmentDate()).isEqualTo(LocalDate.now().plusMonths(1));
        assertThat(createdLoan.getRemainingDebt()).isEqualByComparingTo(request.getAmount());

        ArgumentCaptor<Installment> installmentCaptor = ArgumentCaptor.forClass(Installment.class);
        verify(installmentRepository).save(installmentCaptor.capture());
        Installment createdInstallment = installmentCaptor.getValue();
        assertThat(createdInstallment.getPaymentStatus()).isEqualTo(PaymentStatus.UNPAID);
        assertThat(createdInstallment.getLoan()).isSameAs(createdLoan);

        TransactionSynchronization synchronization = TransactionSynchronizationManager.getSynchronizations().getFirst();
        synchronization.afterCommit();

        ArgumentCaptor<EmailDto> emailCaptor = ArgumentCaptor.forClass(EmailDto.class);
        verify(rabbitClient).sendEmailNotification(emailCaptor.capture());
        assertThat(emailCaptor.getValue().getEmailType()).isEqualTo(EmailType.CREDIT_APPROVED);
        assertThat(emailCaptor.getValue().getApprovedAmount()).isEqualByComparingTo(request.getAmount());
    }

    @Test
    void confirmationDeclineSendsDeclineEmailWithoutCreatingLoan() {
        LoanRequest request = persistedRequest(Status.PENDING);
        when(loanRequestRepository.updateStatus(15L, Status.DECLINED)).thenReturn(1);
        when(loanRequestRepository.findById(15L)).thenReturn(Optional.of(request));

        TransactionSynchronizationManager.initSynchronization();

        String result = service.confirmation(jwt(999L, "BASIC"), 15L, Status.DECLINED);

        assertThat(result).isEqualTo("ODBIJEN ZAHTEV");
        verify(loanRepository, never()).save(any());
        verify(installmentRepository, never()).save(any());

        TransactionSynchronizationManager.getSynchronizations().getFirst().afterCommit();

        ArgumentCaptor<EmailDto> emailCaptor = ArgumentCaptor.forClass(EmailDto.class);
        verify(rabbitClient).sendEmailNotification(emailCaptor.capture());
        assertThat(emailCaptor.getValue().getEmailType()).isEqualTo(EmailType.CREDIT_DENIED);
        assertThat(emailCaptor.getValue().getCreditId()).isEqualTo(request.getClientId());
    }

    @Test
    void confirmationRejectsUnsupportedStatus() {
        assertThatThrownBy(() -> service.confirmation(jwt(999L, "BASIC"), 15L, Status.PENDING))
                .isInstanceOf(IllegalArgumentException.class)
                .hasMessageContaining("approved ili declined");

        verify(loanRequestRepository, never()).updateStatus(any(), any());
    }

    @Test
    void confirmationThrowsWhenRequestDoesNotExist() {
        when(loanRequestRepository.updateStatus(999L, Status.APPROVED)).thenReturn(0);
        when(loanRequestRepository.findById(999L)).thenReturn(Optional.empty());

        assertThatThrownBy(() -> service.confirmation(jwt(999L, "BASIC"), 999L, Status.APPROVED))
                .isInstanceOf(RuntimeException.class)
                .hasMessageContaining("Ne postoji loanRequest");
    }

    @Test
    void confirmationThrowsWhenRequestAlreadyProcessed() {
        LoanRequest request = persistedRequest(Status.APPROVED);
        when(loanRequestRepository.updateStatus(15L, Status.APPROVED)).thenReturn(0);
        when(loanRequestRepository.findById(15L)).thenReturn(Optional.of(request));

        assertThatThrownBy(() -> service.confirmation(jwt(999L, "BASIC"), 15L, Status.APPROVED))
                .isInstanceOf(RuntimeException.class)
                .hasMessageContaining("Umesto PENDING status je: APPROVED");
    }

    @Test
    void infoReturnsLoanDetailsToOwner() {
        Loan loan = activeLoan(77L);
        Installment installment = new Installment(loan, new BigDecimal("12222.33"), new BigDecimal("0.0060"),
                CurrencyCode.RSD, LocalDate.now().plusMonths(1), null, PaymentStatus.UNPAID);
        installment.setId(501L);
        loan.setInstallments(List.of(installment));
        when(loanRepository.findById(loan.getId())).thenReturn(Optional.of(loan));

        LoanInfoResponseDto info = service.info(jwt(77L, "CLIENT_BASIC"), loan.getId());

        assertThat(info.getLoan().getLoanNumber()).isEqualTo(loan.getId());
        assertThat(info.getLoan().getStatus()).isEqualTo(Status.ACTIVE);
        assertThat(info.getInstallments()).hasSize(1);
        assertThat(info.getInstallments().getFirst().getPaymentStatus()).isEqualTo(PaymentStatus.UNPAID);
    }

    @Test
    void infoRejectsNonOwnerWithoutEmployeeRole() {
        Loan loan = activeLoan(77L);
        when(loanRepository.findById(loan.getId())).thenReturn(Optional.of(loan));

        assertThatThrownBy(() -> service.info(jwt(88L, "CLIENT_BASIC"), loan.getId()))
                .isInstanceOf(IllegalArgumentException.class)
                .hasMessageContaining("Nemas dozvolu");
    }

    @Test
    void infoAllowsEmployeeRoleForNonOwner() {
        Loan loan = activeLoan(77L);
        loan.setInstallments(List.of());
        when(loanRepository.findById(loan.getId())).thenReturn(Optional.of(loan));

        LoanInfoResponseDto info = service.info(jwt(88L, "BASIC"), loan.getId());

        assertThat(info.getLoan().getLoanNumber()).isEqualTo(loan.getId());
        assertThat(info.getInstallments()).isEmpty();
    }

    @Test
    void infoThrowsWhenLoanMissing() {
        when(loanRepository.findById(321L)).thenReturn(Optional.empty());

        assertThatThrownBy(() -> service.info(jwt(77L, "CLIENT_BASIC"), 321L))
                .isInstanceOf(IllegalArgumentException.class)
                .hasMessageContaining("Ne postoji loan");
    }

    @Test
    void cronForRatesMarksInstallmentPaidAndGeneratesNextInstallment() {
        Loan loan = activeLoan(77L);
        loan.setRepaymentPeriod(12);
        loan.setInstallmentCount(0);
        loan.setRemainingDebt(new BigDecimal("120000.00"));
        Installment installment = new Installment(loan, new BigDecimal("10500.00"), new BigDecimal("0.0050"),
                CurrencyCode.RSD, LocalDate.now(), null, PaymentStatus.UNPAID);

        when(installmentRepository.findInstallmentByExpectedDueDateLessThanEqualAndPaymentStatusNot(eq(LocalDate.now()), eq(PaymentStatus.PAID)))
                .thenReturn(List.of(installment));
        doNothing().when(accountService).transactionFromBank(any(BankPaymentDto.class));

        service.cronForRates();

        assertThat(installment.getPaymentStatus()).isEqualTo(PaymentStatus.PAID);
        assertThat(installment.getActualDueDate()).isEqualTo(LocalDate.now());
        assertThat(loan.getInstallmentCount()).isEqualTo(1);
        assertThat(loan.getRemainingDebt()).isLessThan(new BigDecimal("120000.00"));
        assertThat(loan.getNextInstallmentDate()).isEqualTo(LocalDate.now().plusMonths(1));
        verify(installmentRepository).save(any(Installment.class));
    }

    @Test
    void cronForRatesSchedulesRetryAndFailureEmailWhenBankChargeFails() {
        Loan loan = activeLoan(77L);
        Installment installment = new Installment(loan, new BigDecimal("10500.00"), new BigDecimal("0.0050"),
                CurrencyCode.RSD, LocalDate.now(), null, PaymentStatus.UNPAID);
        installment.setRetry(0);

        when(installmentRepository.findInstallmentByExpectedDueDateLessThanEqualAndPaymentStatusNot(eq(LocalDate.now()), eq(PaymentStatus.PAID)))
                .thenReturn(List.of(installment));
        doThrow(new HttpClientErrorException(HttpStatus.BAD_REQUEST))
                .when(accountService).transactionFromBank(any(BankPaymentDto.class));

        TransactionSynchronizationManager.initSynchronization();

        service.cronForRates();

        assertThat(installment.getRetry()).isEqualTo(1);
        assertThat(installment.getPaymentStatus()).isEqualTo(PaymentStatus.UNPAID);
        assertThat(installment.getExpectedDueDate()).isEqualTo(LocalDate.now().plusDays(3));
        assertThat(loan.getNextInstallmentDate()).isEqualTo(LocalDate.now().plusDays(3));

        TransactionSynchronizationManager.getSynchronizations().getFirst().afterCommit();

        ArgumentCaptor<EmailDto> emailCaptor = ArgumentCaptor.forClass(EmailDto.class);
        verify(rabbitClient).sendEmailNotification(emailCaptor.capture());
        assertThat(emailCaptor.getValue().getEmailType()).isEqualTo(EmailType.CREDIT_INSTALLMENT_FAILED);
        assertThat(emailCaptor.getValue().getHours()).isEqualTo(72);
    }

    @Test
    void cronForRatesMarksOverdueOnSecondFailedRetry() {
        Loan loan = activeLoan(77L);
        Installment installment = new Installment(loan, new BigDecimal("10500.00"), new BigDecimal("0.0050"),
                CurrencyCode.RSD, LocalDate.now(), null, PaymentStatus.UNPAID);
        installment.setRetry(1);

        when(installmentRepository.findInstallmentByExpectedDueDateLessThanEqualAndPaymentStatusNot(eq(LocalDate.now()), eq(PaymentStatus.PAID)))
                .thenReturn(List.of(installment));
        doThrow(new HttpClientErrorException(HttpStatus.BAD_REQUEST))
                .when(accountService).transactionFromBank(any(BankPaymentDto.class));

        TransactionSynchronizationManager.initSynchronization();

        service.cronForRates();

        assertThat(installment.getPaymentStatus()).isEqualTo(PaymentStatus.OVERDUE);
        assertThat(loan.getStatus()).isEqualTo(Status.OVERDUE);
        assertThat(installment.getExpectedDueDate()).isEqualTo(LocalDate.now().plusDays(1));
        assertThat(loan.getNextInstallmentDate()).isEqualTo(LocalDate.now().plusDays(1));

        TransactionSynchronizationManager.getSynchronizations().getFirst().afterCommit();

        ArgumentCaptor<EmailDto> emailCaptor = ArgumentCaptor.forClass(EmailDto.class);
        verify(rabbitClient).sendEmailNotification(emailCaptor.capture());
        assertThat(emailCaptor.getValue().getEmailType()).isEqualTo(EmailType.CREDIT_INSTALLMENT_FAILED);
        assertThat(emailCaptor.getValue().getHours()).isEqualTo(24);
    }

    @Test
    void cronForRatesMarksLoanPaidOffWhenDebtCleared() {
        Loan loan = activeLoan(77L);
        loan.setRepaymentPeriod(3);
        loan.setInstallmentCount(2);
        loan.setRemainingDebt(new BigDecimal("1000.00"));
        Installment installment = new Installment(loan, new BigDecimal("1100.00"), BigDecimal.ZERO,
                CurrencyCode.RSD, LocalDate.now(), null, PaymentStatus.UNPAID);

        when(installmentRepository.findInstallmentByExpectedDueDateLessThanEqualAndPaymentStatusNot(eq(LocalDate.now()), eq(PaymentStatus.PAID)))
                .thenReturn(List.of(installment));
        doNothing().when(accountService).transactionFromBank(any(BankPaymentDto.class));

        service.cronForRates();

        assertThat(installment.getPaymentStatus()).isEqualTo(PaymentStatus.PAID);
        assertThat(loan.getStatus()).isEqualTo(Status.PAID_OFF);
        verify(installmentRepository, never()).save(any(Installment.class));
    }

    @Test
    void findReturnsMappedPageForClient() {
        Loan lower = activeLoan(77L);
        lower.setId(101L);
        lower.setAmount(new BigDecimal("5000.00"));
        Loan higher = activeLoan(77L);
        higher.setId(102L);
        higher.setAmount(new BigDecimal("9000.00"));

        when(loanRepository.findByClientIdOrderByAmountDesc(77L, PageRequest.of(0, 2)))
                .thenReturn(new PageImpl<>(List.of(higher, lower), PageRequest.of(0, 2), 2));

        PageImpl<LoanResponseDto> page = (PageImpl<LoanResponseDto>) service.find(jwt(77L, "CLIENT_BASIC"), 0, 2);

        assertThat(page.getTotalElements()).isEqualTo(2);
        assertThat(page.getContent().getFirst().getLoanNumber()).isEqualTo(102L);
        assertThat(page.getContent().getFirst().getAmount()).isEqualByComparingTo("9000.00");
    }

    @Test
    void findAllLoanRequestDelegatesFiltersToRepository() {
        LoanRequest request = persistedRequest(Status.PENDING);
        when(loanRequestRepository.findAllWithFilters(LoanType.AUTO, "ACC-001", PageRequest.of(0, 5)))
                .thenReturn(new PageImpl<>(List.of(request), PageRequest.of(0, 5), 1));

        PageImpl<LoanRequest> page = (PageImpl<LoanRequest>) service.findAllLoanRequest(jwt(999L, "BASIC"), LoanType.AUTO, "ACC-001", 0, 5);

        assertThat(page.getTotalElements()).isEqualTo(1);
        assertThat(page.getContent().getFirst().getId()).isEqualTo(request.getId());
    }

    @Test
    void findAllLoansDelegatesFiltersAndMapsResponse() {
        Loan loan = activeLoan(77L);
        when(loanRepository.findAllWithFilters(LoanType.AUTO, "ACC-001", Status.ACTIVE, PageRequest.of(0, 10)))
                .thenReturn(new PageImpl<>(List.of(loan), PageRequest.of(0, 10), 1));

        PageImpl<LoanResponseDto> page = (PageImpl<LoanResponseDto>) service.findAllLoans(jwt(999L, "BASIC"), LoanType.AUTO, "ACC-001", Status.ACTIVE, 0, 10);

        assertThat(page.getTotalElements()).isEqualTo(1);
        assertThat(page.getContent().getFirst().getLoanNumber()).isEqualTo(loan.getId());
        assertThat(page.getContent().getFirst().getStatus()).isEqualTo(Status.ACTIVE);
    }

    private LoanRequestDto validRequestDto() {
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
                "ACC-001"
        );
    }

    private LoanRequest persistedRequest(Status status) {
        LoanRequest request = new LoanRequest(
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
                status,
                "pera@test.com",
                "pera"
        );
        request.setId(15L);
        request.setCreatedAt(LocalDateTime.now().minusDays(1));
        return request;
    }

    private Loan activeLoan(Long clientId) {
        Loan loan = new Loan();
        loan.setId(100L);
        loan.setLoanType(LoanType.AUTO);
        loan.setAccountNumber("ACC-001");
        loan.setAmount(new BigDecimal("500000.00"));
        loan.setRepaymentPeriod(24);
        loan.setNominalInterestRate(new BigDecimal("0.0060"));
        loan.setEffectiveInterestRate(new BigDecimal("0.0060"));
        loan.setInterestType(InterestType.FIXED);
        loan.setAgreementDate(LocalDate.now().minusMonths(2));
        loan.setMaturityDate(LocalDate.now().plusMonths(22));
        loan.setInstallmentAmount(new BigDecimal("22000.00"));
        loan.setNextInstallmentDate(LocalDate.now().plusDays(1));
        loan.setRemainingDebt(new BigDecimal("420000.00"));
        loan.setCurrency(CurrencyCode.RSD);
        loan.setStatus(Status.ACTIVE);
        loan.setUserEmail("pera@test.com");
        loan.setUsername("pera");
        loan.setClientId(clientId);
        loan.setInstallmentCount(2);
        return loan;
    }

    private InternalAccountDetailsDto accountDetails(Long ownerId, CurrencyCode currency, String email, String username) {
        return new InternalAccountDetailsDto(
                null,
                "ACC-001",
                ownerId,
                currency.name(),
                null,
                null,
                null,
                email,
                username
        );
    }

    private Jwt jwt(long id, String roles) {
        return new Jwt(
                "token",
                LocalDateTime.now().minusMinutes(1).toInstant(java.time.ZoneOffset.UTC),
                LocalDateTime.now().plusHours(1).toInstant(java.time.ZoneOffset.UTC),
                Map.of("alg", "none"),
                Map.of("id", id, "roles", roles)
        );
    }
}
