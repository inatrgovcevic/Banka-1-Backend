package http

import (
	"net/http"
	"strings"
)

// authMode opisuje koji nivo autorizacije endpoint zahteva — mapirano iz
// Java security-lib SecurityConfig (apiChain: anyRequest().authenticated())
// i @PreAuthorize anotacija po kontroleru.
type authMode int

const (
	authOpen          authMode = iota // permit-all (bez JWT-a)
	authAuthenticated                 // bilo koji validan principal
	authRoles                         // jedna od navedenih rola (uz role hijerarhiju)
)

// Role hijerarhija iz security-lib SecurityConfig.roleHierarchy():
//
//	ROLE_ADMIN > ROLE_SUPERVISOR > ROLE_AGENT > ROLE_BASIC
//	ROLE_CLIENT_TRADING > ROLE_CLIENT_BASIC
//
// roleImplies[r] daje role koje r direktno "pokriva"; tranzitivno zatvaranje
// se racuna u expandRole.
var roleImplies = map[string][]string{
	"ADMIN":          {"SUPERVISOR"},
	"SUPERVISOR":     {"AGENT"},
	"AGENT":          {"BASIC"},
	"CLIENT_TRADING": {"CLIENT_BASIC"},
}

var (
	rolesService            = []string{"SERVICE"}
	rolesServiceBasic       = []string{"SERVICE", "BASIC"}
	rolesBasic              = []string{"BASIC"}
	rolesBasicService       = []string{"BASIC", "SERVICE"}
	rolesCurrency           = []string{"CLIENT_BASIC", "BASIC"}
	rolesClientOrBasic      = []string{"CLIENT_BASIC", "BASIC"}
	rolesClientAgent        = []string{"CLIENT_BASIC", "AGENT"}
	rolesClientAgentService = []string{"CLIENT_BASIC", "AGENT", "SERVICE"}
	rolesServiceAdmin       = []string{"SERVICE", "ADMIN"}
	rolesClientAdmin        = []string{"CLIENT_BASIC", "ADMIN"}
	rolesClientBasic        = []string{"CLIENT_BASIC"}
	rolesTxEmployee         = []string{"ADMIN", "SUPERVISOR", "AGENT", "BASIC"}
	rolesTransfer           = []string{"CLIENT_BASIC", "ADMIN", "SERVICE"}
)

// enforceAuth sprovodi autorizaciju pre dispatch-a. Vraca false (i upisuje
// gresku) ako zahtev nije autorizovan.
func (h *Handler) enforceAuth(w http.ResponseWriter, r *http.Request) bool {
	mode, roles := routeAuth(r.Method, r.URL.Path)
	switch mode {
	case authOpen:
		return true
	case authRoles:
		return h.requireAnyRole(w, r, roles...)
	default:
		return h.requireAuthenticated(w, r)
	}
}

// requireAnyRole proverava da pozivalac poseduje bar jednu od dozvoljenih rola
// (uz role hijerarhiju). 401 ako nije autentifikovan, 403 ako rola ne odgovara.
func (h *Handler) requireAnyRole(w http.ResponseWriter, r *http.Request, allowed ...string) bool {
	roles, ok := h.authenticatedRoles(w, r)
	if !ok {
		return false
	}
	if hasAnyRoleHierarchical(roles, allowed) {
		return true
	}
	writeError(w, http.StatusForbidden, "ERR_FORBIDDEN", "Pristup odbijen", "Nedovoljna prava za pristup resursu")
	return false
}

// authenticatedRoles validira identitet (JWT ili gateway header-i) i vraca role.
// NE zahteva numericki subject id — servisni tokeni (sub="banking-core-service")
// ga nemaju, a Java hasRole(...) proverava samo authority.
func (h *Handler) authenticatedRoles(w http.ResponseWriter, r *http.Request) ([]string, bool) {
	if token := bearerToken(r); token != "" {
		claims, ok := h.verifiedJWTClaims(token)
		if !ok {
			writeError(w, http.StatusUnauthorized, "ERR_UNAUTHORIZED", "Pristup odbijen", "Neispravan JWT token")
			return nil, false
		}
		return h.rolesFromClaims(claims), true
	}
	if roles := rolesFromHeader(r); len(roles) > 0 {
		return roles, true
	}
	writeError(w, http.StatusUnauthorized, "ERR_UNAUTHORIZED", "Pristup odbijen", "Potrebna je autentifikacija")
	return nil, false
}

func hasAnyRoleHierarchical(held, allowed []string) bool {
	expanded := map[string]bool{}
	for _, role := range held {
		expandRole(normalizeRoleName(role), expanded)
	}
	for _, role := range allowed {
		if expanded[normalizeRoleName(role)] {
			return true
		}
	}
	return false
}

func expandRole(role string, out map[string]bool) {
	if role == "" || out[role] {
		return
	}
	out[role] = true
	for _, implied := range roleImplies[role] {
		expandRole(implied, out)
	}
}

func normalizeRoleName(role string) string {
	return strings.TrimPrefix(strings.ToUpper(strings.TrimSpace(role)), "ROLE_")
}

// routeAuth mapira (metod, putanja) na zahtevani nivo autorizacije. Ogledalo je
// dispatch switch-a u handler.go i konsolidovane @PreAuthorize mape iz Jave.
func routeAuth(method, path string) (authMode, []string) {
	g := method == http.MethodGet
	pu := method == http.MethodPut
	has := strings.HasPrefix
	suf := strings.HasSuffix

	switch {
	// --- permit-all: actuator probe-ovi/metrike i interni account lookup ---
	case has(path, "/actuator/"):
		return authOpen, nil
	case has(path, "/accounts/internal/"):
		return authOpen, nil

	// --- verifikacija: samo authenticated (nema @PreAuthorize) ---
	case has(path, "/verification/"):
		return authAuthenticated, nil

	// --- currencies: CLIENT_BASIC,BASIC ---
	case has(path, "/accounts/api/currencies"):
		return authRoles, rolesCurrency

	// --- employee accounts: BASIC (osim GET /accounts/employee/accounts -> BASIC,SERVICE) ---
	case has(path, "/accounts/employee/"):
		if g && path == "/accounts/employee/accounts" {
			return authRoles, rolesBasicService
		}
		return authRoles, rolesBasic

	// --- client accounts: CLIENT_BASIC,AGENT (GET /api/accounts/{num} dodaje SERVICE) ---
	case has(path, "/accounts/client/"):
		if g && has(path, "/accounts/client/api/accounts/") && !suf(path, "/name") && !suf(path, "/limits") {
			return authRoles, rolesClientAgentService
		}
		return authRoles, rolesClientAgent

	// --- margin racuni pod /accounts: samo authenticated ---
	case path == "/accounts/createMarginAccount",
		path == "/accounts/company/createMarginAccount",
		has(path, "/accounts/getMarginUser/"),
		has(path, "/accounts/company/getMarginCompany/"):
		return authAuthenticated, nil

	// --- kartice ---
	case path == "/api/cards/auto":
		return authRoles, rolesServiceAdmin
	case path == "/api/cards/request", path == "/api/cards/request/business":
		return authRoles, rolesClientAdmin
	case has(path, "/api/cards/internal/account/"):
		return authRoles, rolesService
	case has(path, "/api/cards/client/"):
		return authRoles, rolesClientOrBasic
	case has(path, "/api/cards/account/"):
		return authRoles, rolesBasic
	case path == "/api/cards/all":
		return authRoles, rolesBasic
	case has(path, "/api/cards/id/"):
		if pu && (suf(path, "/unblock") || suf(path, "/deactivate")) {
			return authRoles, rolesBasic
		}
		return authRoles, rolesClientOrBasic // block, limit (PUT), GET id/{id}

	// --- transakcije ---
	case has(path, "/transactions/internal/"):
		return authRoles, rolesService
	case path == "/transactions/payment", path == "/transactions/payments":
		return authRoles, rolesClientBasic
	case path == "/transactions/by-client",
		path == "/transactions/by-sender-client",
		path == "/transactions/by-recipient-client":
		return authRoles, rolesBasic
	case path == "/transactions/by-this-client",
		path == "/transactions/by-this-sender-client",
		path == "/transactions/by-this-recipient-client":
		return authRoles, rolesClientBasic
	case path == "/transactions/api/payments":
		return authRoles, rolesClientOrBasic
	case has(path, "/transactions/employee/accounts/"):
		return authRoles, rolesTxEmployee
	case has(path, "/transactions/accounts/"):
		return authRoles, rolesClientOrBasic
	case path == "/transactions/stockBuyMarginTransaction",
		path == "/transactions/stockSellMarginTransaction",
		has(path, "/transactions/addToMargin/"),
		has(path, "/transactions/withdrawFromMargin/"),
		has(path, "/transactions/getAllMarginTransactions/"):
		return authAuthenticated, nil

	// --- payment recipients: CLIENT_BASIC ---
	case path == "/payment-recipients", has(path, "/payment-recipients/"):
		return authRoles, rolesClientBasic

	// --- transferi: CLIENT_BASIC,ADMIN,SERVICE ---
	case path == "/transfers", path == "/transfers/", has(path, "/transfers/"):
		return authRoles, rolesTransfer

	// --- interbank: SERVICE ---
	case has(path, "/internal/interbank/"):
		return authRoles, rolesService

	// --- AccountController /internal/accounts: SERVICE (system -> SERVICE,BASIC) ---
	case path == "/internal/accounts/system":
		return authRoles, rolesServiceBasic
	case has(path, "/internal/accounts/"):
		return authRoles, rolesService

	default:
		// apiChain: anyRequest().authenticated()
		return authAuthenticated, nil
	}
}
