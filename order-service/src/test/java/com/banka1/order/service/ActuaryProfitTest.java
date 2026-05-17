package com.banka1.order.service;

import com.banka1.order.client.EmployeeClient;
import com.banka1.order.dto.ActuaryProfitDto;
import com.banka1.order.dto.BankProfitSummaryDto;
import com.banka1.order.dto.EmployeeDto;
import com.banka1.order.repository.ActuaryInfoRepository;
import com.banka1.order.repository.TransactionRepository;
import com.banka1.order.service.impl.ActuaryServiceImpl;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.InjectMocks;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;

import java.math.BigDecimal;
import java.time.LocalDateTime;
import java.util.List;

import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.anyLong;
import static org.mockito.Mockito.lenient;
import static org.mockito.Mockito.when;

/**
 * Unit testovi za PR_14 C14.9 (profitByActuary) i PR_17 C17.6 (bankProfitSummary).
 */
@ExtendWith(MockitoExtension.class)
class ActuaryProfitTest {

    @Mock private ActuaryInfoRepository actuaryInfoRepository;
    @Mock private EmployeeClient employeeClient;
    @Mock private TransactionRepository transactionRepository;

    @InjectMocks private ActuaryServiceImpl service;

    @Test
    void profitByActuary_mapira_red_iz_query_u_dto_i_enrich_uje_imena() {
        when(transactionRepository.sumCommissionByActuary(any(), any())).thenReturn(List.of(
                row(1L, new BigDecimal("12500.00"), 30L),
                row(2L, new BigDecimal("8000.00"), 20L)
        ));

        EmployeeDto e1 = new EmployeeDto();
        e1.setId(1L); e1.setIme("Marko"); e1.setPrezime("Markovic"); e1.setPozicija("Agent");
        EmployeeDto e2 = new EmployeeDto();
        e2.setId(2L); e2.setIme("Ana"); e2.setPrezime("Anic"); e2.setPozicija("Supervisor");
        when(employeeClient.getEmployee(1L)).thenReturn(e1);
        when(employeeClient.getEmployee(2L)).thenReturn(e2);

        List<ActuaryProfitDto> result = service.profitByActuary(null, null);

        assertThat(result).hasSize(2);
        assertThat(result.get(0).getUserId()).isEqualTo(1L);
        assertThat(result.get(0).getTotalCommission()).isEqualByComparingTo("12500.00");
        assertThat(result.get(0).getTransactionCount()).isEqualTo(30L);
        assertThat(result.get(0).getIme()).isEqualTo("Marko");
        assertThat(result.get(0).getPrezime()).isEqualTo("Markovic");
        assertThat(result.get(0).getPozicija()).isEqualTo("Agent");
    }

    @Test
    void profitByActuary_tolerise_employee_service_failure() {
        when(transactionRepository.sumCommissionByActuary(any(), any())).thenReturn(List.of(
                row(1L, new BigDecimal("12500.00"), 30L)
        ));
        when(employeeClient.getEmployee(anyLong())).thenThrow(new RuntimeException("employee-service down"));

        List<ActuaryProfitDto> result = service.profitByActuary(null, null);

        // Komisija + count se i dalje vracaju, samo bez imena.
        assertThat(result).hasSize(1);
        assertThat(result.get(0).getTotalCommission()).isEqualByComparingTo("12500.00");
        assertThat(result.get(0).getIme()).isNull();
        assertThat(result.get(0).getPrezime()).isNull();
    }

    @Test
    void profitByActuary_handles_null_commission_row() {
        when(transactionRepository.sumCommissionByActuary(any(), any())).thenReturn(List.of(
                row(1L, null, null)
        ));
        lenient().when(employeeClient.getEmployee(anyLong())).thenReturn(null);

        List<ActuaryProfitDto> result = service.profitByActuary(null, null);

        assertThat(result).hasSize(1);
        assertThat(result.get(0).getTotalCommission()).isEqualByComparingTo("0");
        assertThat(result.get(0).getTransactionCount()).isZero();
    }

    @Test
    void bankProfitSummary_sabira_komisije_preko_svih_aktuara() {
        when(transactionRepository.sumCommissionByActuary(any(), any())).thenReturn(List.of(
                row(1L, new BigDecimal("12500.00"), 30L),
                row(2L, new BigDecimal("8000.00"), 20L),
                row(3L, new BigDecimal("4500.50"), 10L)
        ));
        // employeeClient se zove kroz profitByActuary; vracamo null tolerantno
        lenient().when(employeeClient.getEmployee(anyLong())).thenReturn(null);

        LocalDateTime from = LocalDateTime.of(2025, 1, 1, 0, 0);
        LocalDateTime to = LocalDateTime.of(2025, 12, 31, 23, 59);
        BankProfitSummaryDto summary = service.bankProfitSummary(from, to);

        assertThat(summary.getTotalCommission()).isEqualByComparingTo("25000.50");
        assertThat(summary.getTransactionCount()).isEqualTo(60L);
        assertThat(summary.getDistinctActuaries()).isEqualTo(3L);
        assertThat(summary.getFrom()).isEqualTo(from);
        assertThat(summary.getTo()).isEqualTo(to);
    }

    @Test
    void bankProfitSummary_vraca_nule_kad_nema_transakcija() {
        when(transactionRepository.sumCommissionByActuary(any(), any())).thenReturn(List.of());

        BankProfitSummaryDto summary = service.bankProfitSummary(null, null);

        assertThat(summary.getTotalCommission()).isEqualByComparingTo("0");
        assertThat(summary.getTransactionCount()).isZero();
        assertThat(summary.getDistinctActuaries()).isZero();
    }

    /** Helper za TransactionRepository.ActuaryProfitRow projection. */
    private static TransactionRepository.ActuaryProfitRow row(Long userId, BigDecimal commission, Long count) {
        return new TransactionRepository.ActuaryProfitRow() {
            @Override public Long getUserId() { return userId; }
            @Override public BigDecimal getTotalCommission() { return commission; }
            @Override public Long getTransactionCount() { return count; }
        };
    }
}
