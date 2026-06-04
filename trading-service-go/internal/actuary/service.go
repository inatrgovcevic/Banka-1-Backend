package actuary

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"banka1/trading-service-go/internal/api"
	"banka1/trading-service-go/internal/audit"
	"banka1/trading-service-go/internal/clients"

	"github.com/shopspring/decimal"
)

// employeePageSize mirrors ActuaryServiceImpl.EMPLOYEE_PAGE_SIZE: the page size
// used when sweeping user-service for all agents.
const employeePageSize = 100

// Service implements the /actuaries supervisor operations, merging user-service
// employee data with local actuary_info. Mirrors ActuaryServiceImpl.
type Service struct {
	repo      *Repository
	employees *clients.EmployeeClient
	auditor   *audit.Service
}

func NewService(repo *Repository, employees *clients.EmployeeClient, auditor *audit.Service) *Service {
	return &Service{repo: repo, employees: employees, auditor: auditor}
}

// GetAgents returns a Spring-style page of AGENT employees merged with their
// actuary_info. It sweeps all user-service pages, filters to AGENT, dedups by
// employeeId preserving first-seen order, then slices the requested page.
func (s *Service) GetAgents(ctx context.Context, email, ime, prezime, pozicija *string, page, size int) (api.Page[api.ActuaryAgentDto], error) {
	// Single-term search: when ime == prezime, search each as first then last
	// name and merge (mirrors the singleTermSearch branch).
	singleTerm := ime != nil && prezime != nil && *ime == *prezime

	seen := map[int64]bool{}
	agents := make([]api.ActuaryAgentDto, 0)

	collect := func(email, ime, prezime, pozicija *string) error {
		pageIndex := 0
		for {
			pg, err := s.employees.SearchEmployees(ctx, email, ime, prezime, pozicija, pageIndex, employeePageSize)
			if err != nil {
				return err
			}
			if pg == nil || len(pg.Content) == 0 {
				break
			}
			for _, emp := range pg.Content {
				if !roleMatches(emp.Role, "AGENT") || seen[emp.ID] {
					continue
				}
				info, err := s.repo.FindOrCreate(ctx, emp.ID)
				if err != nil {
					return err
				}
				agents = append(agents, toDto(emp, info))
				seen[emp.ID] = true
			}
			pageIndex++
			if pageIndex >= pg.TotalPages {
				break
			}
		}
		return nil
	}

	if singleTerm {
		if err := collect(email, ime, nil, pozicija); err != nil {
			return api.Page[api.ActuaryAgentDto]{}, err
		}
		if err := collect(email, nil, prezime, pozicija); err != nil {
			return api.Page[api.ActuaryAgentDto]{}, err
		}
	} else if err := collect(email, ime, prezime, pozicija); err != nil {
		return api.Page[api.ActuaryAgentDto]{}, err
	}

	total := len(agents)
	slice := make([]api.ActuaryAgentDto, 0)
	if start := page * size; start < total {
		end := start + size
		if end > total {
			end = total
		}
		slice = agents[start:end]
	}
	return api.NewPage(slice, page, size, int64(total)), nil
}

// SetLimit sets an agent's daily limit. limit is already bean-validated (>0,
// non-null) by the handler. Mirrors ActuaryServiceImpl.setLimit.
func (s *Service) SetLimit(ctx context.Context, actorID int64, actorRole string, employeeID int64, limit decimal.Decimal) error {
	emp, err := s.fetchEmployeeOrNotFound(ctx, employeeID)
	if err != nil {
		return err
	}
	if roleMatches(emp.Role, "ADMIN") {
		return api.NewOtcError(404, "Cannot change the limit of an admin.")
	}
	if !roleMatches(emp.Role, "AGENT") {
		return api.NewOtcError(404, "Limit can only be set for employees with the AGENT role.")
	}
	info, err := s.repo.FindOrCreate(ctx, employeeID)
	if err != nil {
		return err
	}
	if limit.LessThan(info.UsedLimit) {
		return api.NewOtcError(404, "Limit cannot be lower than the current used limit of "+info.UsedLimit.String()+".")
	}
	oldValue := "-"
	if info.Limit != nil {
		oldValue = info.Limit.String()
	}
	if err := s.repo.UpdateLimit(ctx, employeeID, limit); err != nil {
		return err
	}
	// WP-2 audit trail, Ilijan's AuditActionType naming: the legacy
	// SET_LIMIT/old_value/new_value stub write became an AGENT_LIMIT_CHANGED
	// event with the change captured in details.
	s.recordAgentAudit(ctx, actorID, audit.ActionAgentLimitChanged, employeeID,
		"Limit promenjen: "+oldValue+" -> "+limit.String())
	return nil
}

// ResetLimit zeroes an agent's used/reserved limit. Mirrors resetLimit.
func (s *Service) ResetLimit(ctx context.Context, actorID int64, actorRole string, employeeID int64) error {
	emp, err := s.fetchEmployeeOrNotFound(ctx, employeeID)
	if err != nil {
		return err
	}
	if roleMatches(emp.Role, "ADMIN") {
		return api.NewOtcError(404, "Cannot reset the limit of an admin.")
	}
	if !roleMatches(emp.Role, "AGENT") {
		return api.NewOtcError(404, "Limit can only be reset for employees with the AGENT role.")
	}
	info, _ := s.repo.FindByEmployeeID(ctx, employeeID)
	oldUsed := "0"
	if info != nil {
		oldUsed = info.UsedLimit.String()
	}
	if err := s.repo.ResetLimit(ctx, employeeID); err != nil {
		return err
	}
	s.recordAgentAudit(ctx, actorID, audit.ActionAgentUsedLimitReset, employeeID,
		"Iskorisceni limit resetovan: "+oldUsed+" -> 0")
	return nil
}

// recordAgentAudit records an agent-management audit event (WP-2) with the
// actor's display name resolved from user-service (falling back to the raw id,
// mirroring resolveActorName). Best-effort — audit never breaks the operation.
func (s *Service) recordAgentAudit(ctx context.Context, actorID int64, actionType string, employeeID int64, details string) {
	if s.auditor == nil {
		return
	}
	actorName := fmt.Sprintf("%d", actorID)
	if emp, err := s.employees.GetEmployee(ctx, actorID); err == nil && emp != nil {
		parts := []string{}
		if emp.Ime != nil {
			parts = append(parts, strings.TrimSpace(*emp.Ime))
		}
		if emp.Prezime != nil {
			parts = append(parts, strings.TrimSpace(*emp.Prezime))
		}
		if name := strings.TrimSpace(strings.Join(parts, " ")); name != "" {
			actorName = name
		}
	}
	targetType := "EMPLOYEE"
	targetID := fmt.Sprintf("%d", employeeID)
	ts := time.Now().UnixMilli()
	id := actorID
	s.auditor.RecordBestEffort(ctx, audit.Event{
		ActorID:    &id,
		ActorName:  &actorName,
		ActionType: actionType,
		TargetType: &targetType,
		TargetID:   &targetID,
		Details:    &details,
		Timestamp:  &ts,
	})
}

// SetNeedApproval toggles an agent's need-approval flag. value is bean-validated
// non-null by the handler. Mirrors setNeedApproval.
func (s *Service) SetNeedApproval(ctx context.Context, employeeID int64, value bool) error {
	emp, err := s.fetchEmployeeOrNotFound(ctx, employeeID)
	if err != nil {
		return err
	}
	if roleMatches(emp.Role, "ADMIN") {
		return api.NewOtcError(404, "Cannot change the need-approval flag of an admin.")
	}
	if !roleMatches(emp.Role, "AGENT") {
		return api.NewOtcError(404, "The need-approval flag can only be set for employees with the AGENT role.")
	}
	return s.repo.SetNeedApproval(ctx, employeeID, value)
}

// ProfitByActuary returns per-actuary commission totals. Null from/to are
// replaced with sentinels (1900-01-01 / 9999-12-31T23:59) as in Java, since
// Postgres cannot infer the type of a null bound. Names are enriched
// best-effort from user-service.
func (s *Service) ProfitByActuary(ctx context.Context, from, to *time.Time) ([]api.ActuaryProfitDto, error) {
	effFrom := time.Date(1900, 1, 1, 0, 0, 0, 0, time.UTC)
	if from != nil {
		effFrom = *from
	}
	effTo := time.Date(9999, 12, 31, 23, 59, 0, 0, time.UTC)
	if to != nil {
		effTo = *to
	}

	rows, err := s.repo.SumCommissionByActuary(ctx, effFrom, effTo)
	if err != nil {
		return nil, err
	}
	byUser := make(map[int64]api.ActuaryProfitDto, len(rows))
	for _, row := range rows {
		dto := api.ActuaryProfitDto{
			UserID:           row.UserID,
			TotalCommission:  row.TotalCommission,
			TransactionCount: row.TransactionCount,
		}
		// Best-effort name enrichment; any failure leaves the name fields null.
		if emp, err := s.employees.GetEmployee(ctx, row.UserID); err == nil && emp != nil {
			dto.Ime = emp.Ime
			dto.Prezime = emp.Prezime
			dto.Pozicija = emp.Pozicija
		}
		byUser[row.UserID] = dto
	}

	for page := 0; ; page++ {
		employees, err := s.employees.SearchEmployees(ctx, nil, nil, nil, nil, page, employeePageSize)
		if err != nil {
			break
		}
		for _, emp := range employees.Content {
			if !roleMatches(emp.Role, "AGENT") && !roleMatches(emp.Role, "SUPERVISOR") {
				continue
			}
			if _, ok := byUser[emp.ID]; ok {
				continue
			}
			byUser[emp.ID] = api.ActuaryProfitDto{
				UserID:           emp.ID,
				TotalCommission:  decimal.Zero,
				TransactionCount: 0,
				Ime:              emp.Ime,
				Prezime:          emp.Prezime,
				Pozicija:         emp.Pozicija,
			}
		}
		if len(employees.Content) == 0 || page+1 >= employees.TotalPages {
			break
		}
	}

	out := make([]api.ActuaryProfitDto, 0, len(byUser))
	for _, dto := range byUser {
		out = append(out, dto)
	}
	sort.SliceStable(out, func(i, j int) bool {
		cmp := out[i].TotalCommission.Cmp(out[j].TotalCommission)
		if cmp != 0 {
			return cmp > 0
		}
		return out[i].UserID < out[j].UserID
	})
	return out, nil
}

// BankProfitSummary aggregates ProfitByActuary into the bank-wide total. from/to
// echo the original (nullable) query bounds. Mirrors bankProfitSummary.
func (s *Service) BankProfitSummary(ctx context.Context, from, to *time.Time) (api.BankProfitSummaryDto, error) {
	per, err := s.ProfitByActuary(ctx, from, to)
	if err != nil {
		return api.BankProfitSummaryDto{}, err
	}
	total := decimal.Zero
	var txCount int64
	for _, p := range per {
		total = total.Add(p.TotalCommission)
		txCount += p.TransactionCount
	}
	return api.BankProfitSummaryDto{
		TotalCommission:   total,
		TransactionCount:  txCount,
		DistinctActuaries: int64(len(per)),
		From:              api.LocalDateTimeFromPtr(from),
		To:                api.LocalDateTimeFromPtr(to),
	}, nil
}

func (s *Service) fetchEmployeeOrNotFound(ctx context.Context, employeeID int64) (*clients.Employee, error) {
	emp, err := s.employees.GetEmployee(ctx, employeeID)
	if errors.Is(err, clients.ErrNotFound) {
		return nil, api.NewOrderError(404, fmt.Sprintf("Employee with ID %d not found.", employeeID))
	}
	if err != nil {
		return nil, err
	}
	return emp, nil
}

func toDto(emp clients.Employee, info *ActuaryInfo) api.ActuaryAgentDto {
	return api.ActuaryAgentDto{
		EmployeeID:   emp.ID,
		Ime:          emp.Ime,
		Prezime:      emp.Prezime,
		Email:        emp.Email,
		Pozicija:     emp.Pozicija,
		Limit:        info.Limit,
		UsedLimit:    info.UsedLimit,
		NeedApproval: info.NeedApproval,
	}
}

// roleMatches mirrors Role.matches: case-insensitive, trimmed equality.
func roleMatches(role *string, target string) bool {
	return role != nil && strings.EqualFold(strings.TrimSpace(*role), target)
}
