package com.banka1.order.service;

import com.banka1.order.dto.ActuaryAgentDto;
import com.banka1.order.dto.ActuaryProfitDto;
import com.banka1.order.dto.SetLimitRequestDto;
import com.banka1.order.dto.SetNeedApprovalRequestDto;
import org.springframework.data.domain.Page;
import org.springframework.data.domain.Pageable;

import java.time.LocalDateTime;
import java.util.List;

/**
 * Business logic interface for actuary management.
 * All operations are restricted to supervisors (including admins via role hierarchy).
 */
public interface ActuaryService {

    /**
     * Returns a list of all employees with the AGENT role, combined with their actuary limit data.
     * Employee data is fetched from employee-service; limit data from the local database.
     * An {@link com.banka1.order.entity.ActuaryInfo} record is created with default values
     * if one does not yet exist for a given agent.
     *
     * @param email    optional email filter passed to employee-service
     * @param ime      optional first name filter
     * @param prezime  optional last name filter
     * @param pozicija optional position filter
     * @return list of agent DTOs with actuary limit information
     */
    Page<ActuaryAgentDto> getAgents(String email, String ime, String prezime, String pozicija, Pageable pageable);

    /**
     * Updates the daily trading limit for the specified agent.
     * Throws {@link IllegalArgumentException} if the target employee is an ADMIN
     * or does not have the AGENT role.
     *
     * @param employeeId the employee's identifier
     * @param request    new limit value in RSD
     */
    void setLimit(Long employeeId, SetLimitRequestDto request);

    /**
     * Resets the consumed daily limit ({@code usedLimit}) to zero for the specified agent.
     * Throws {@link IllegalArgumentException} if the target employee is an ADMIN.
     *
     * @param employeeId the employee's identifier
     */
    void resetLimit(Long employeeId);

    /**
     * Toggles the {@code needApproval} flag for the specified agent.
     * When set, every order placed by the agent enters the PENDING queue for
     * supervisor approval regardless of whether the order value would otherwise
     * exceed the agent's daily limit.
     * <p>
     * Throws {@link IllegalArgumentException} if the target employee is an ADMIN
     * or does not have the AGENT role (supervisors always have {@code needApproval = false}).
     *
     * @param employeeId the employee's identifier
     * @param request    request body carrying the new flag value
     */
    void setNeedApproval(Long employeeId, SetNeedApprovalRequestDto request);

    /**
     * Resets {@code usedLimit} to zero for every actuary record in the database.
     * Called automatically by the scheduler at 23:59 every day.
     */
    void resetAllLimits();

    /**
     * PR_14 C14.9: vraca trading profit po aktuaru — sumu komisija sa izvrsenih
     * transakcija u zadatom intervalu. {@code from} i {@code to} su opcioni;
     * null = bez donje/gornje granice.
     */
    List<ActuaryProfitDto> profitByActuary(LocalDateTime from, LocalDateTime to);

    /**
     * PR_17 C17.6: agregira trading P&L na nivou banke — ukupna komisija
     * preko svih aktuara i ukupan broj transakcija. Spec (Celina 4.txt — Profit Banke):
     * banka zaradjuje od komisija na izvrsenim trgovinama; ovo je trading-side
     * doprinos profitu. Fund-side (vrednost fondova - ulozeno) se sabira na frontend-u
     * iz {@code GET /funds} endpoint-a.
     */
    com.banka1.order.dto.BankProfitSummaryDto bankProfitSummary(LocalDateTime from, LocalDateTime to);
}
