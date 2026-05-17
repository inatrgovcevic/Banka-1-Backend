package com.banka1.order.dto;

import java.util.Set;

/**
 * Minimal authenticated-user context extracted from the incoming JWT token.
 *
 * This record encapsulates the essential user information needed for authorization
 * and business logic checks. Created from JWT claims and passed through the service
 * layer for permission validation and audit logging.
 *
 * JWT Claims Mapping:
 * <ul>
 *   <li>userId: JWT subject claim (sub) - uniquely identifies the user</li>
 *   <li>roles: JWT roles claim - list of role strings (e.g., "CLIENT_TRADING", "AGENT", "SUPERVISOR")</li>
 *   <li>permissions: JWT permissions claim - list of specific permission strings</li>
 * </ul>
 *
 * Example JWT Claims:
 * <pre>
 * {
 *   "sub": "12345",
 *   "roles": ["CLIENT_TRADING"],
 *   "permissions": ["trading:execute", "margin:use"]
 * }
 * </pre>
 */
public record AuthenticatedUser(Long userId, Set<String> roles, Set<String> permissions) {

    /**
     * Checks if the user has a specific role (case-insensitive).
     *
     * @param role the role name to check (e.g., "CLIENT_TRADING", "AGENT")
     * @return true if the user has this role
     */
    public boolean hasRole(String role) {
        return roles.stream().anyMatch(current -> current.equalsIgnoreCase(role));
    }

    /**
     * Set of permission codes koji daju pravo trgovine sa margin-om.
     * Spec: Marzni_Racuni + Celina 3 — klijent dobija MARGIN_TRADE automatski po
     * odobrenju kredita; aktuari moraju eksplicitno imati ovu permisiju.
     */
    private static final Set<String> MARGIN_PERMISSIONS = Set.of(
            "MARGIN_TRADE", "SECURITIES_TRADE_MARGIN", "MARGIN");

    /** Set permisija/rola koje daju pravo trgovine. */
    private static final Set<String> TRADING_PERMISSIONS = Set.of(
            "SECURITIES_TRADE", "SECURITIES_TRADE_LIMITED", "SECURITIES_TRADE_UNLIMITED",
            "TRADING_BASIC", "TRADING_ADVANCED");

    /**
     * Checks if the user has permission to use margin (borrowed funds).
     * Egzaktno poredi protiv whitelist-a — krhak `contains("margin")` je propusta
     * "VIEW_MARGIN_TRADES" ili sl. permisije koje ne treba da daju trade pravo.
     */
    public boolean hasMarginPermission() {
        return permissions.stream().anyMatch(p -> MARGIN_PERMISSIONS.contains(p.toUpperCase()));
    }

    /**
     * Checks if the user has trading permissions.
     * Users with CLIENT_TRADING role or explicit trading permission can trade.
     */
    public boolean hasTradingPermission() {
        return hasRole("CLIENT_TRADING")
                || permissions.stream().anyMatch(p -> TRADING_PERMISSIONS.contains(p.toUpperCase()));
    }

    /**
     * Checks if the user is a client (not an employee/agent).
     *
     * @return true if user has any CLIENT role
     */
    public boolean isClient() {
        return hasRole("CLIENT_BASIC") || hasRole("CLIENT_TRADING") || hasRole("CLIENT");
    }

    /**
     * Checks if the user is an agent (employee with trading authority).
     *
     * @return true if user has AGENT, SUPERVISOR, or ADMIN role
     */
    public boolean isAgent() {
        return hasRole("AGENT") || hasRole("SUPERVISOR") || hasRole("ADMIN");
    }
}
