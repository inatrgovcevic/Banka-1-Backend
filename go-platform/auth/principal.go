package auth

import "context"

// Principal is the authenticated caller. Mirrors the Java JwtPrincipal.
type Principal struct {
	ID          int64
	Subject     string
	Email       string
	Role        string
	Permissions []string
	Token       string
}

type principalKey struct{}

// WithPrincipal stores p on ctx. Used by Middleware.
func WithPrincipal(ctx context.Context, p Principal) context.Context {
	return context.WithValue(ctx, principalKey{}, p)
}

// PrincipalFromContext returns the Principal attached to ctx, if any.
func PrincipalFromContext(ctx context.Context) (Principal, bool) {
	p, ok := ctx.Value(principalKey{}).(Principal)
	return p, ok
}

// HasAnyRole returns true if any of roles is allowed by Principal.Role taking
// the role hierarchy below into account:
//
//	ADMIN > SUPERVISOR > AGENT > BASIC
//	CLIENT_TRADING > CLIENT_BASIC
//
// SERVICE is its own peer used for service-to-service tokens.
func (p Principal) HasAnyRole(roles ...string) bool {
	granted := grantedRoles(p.Role)
	for _, role := range roles {
		if _, ok := granted[role]; ok {
			return true
		}
	}
	return false
}

// HasAllPermissions returns true if every required permission is on the principal.
func (p Principal) HasAllPermissions(required ...string) bool {
	have := make(map[string]struct{}, len(p.Permissions))
	for _, perm := range p.Permissions {
		have[perm] = struct{}{}
	}
	for _, perm := range required {
		if _, ok := have[perm]; !ok {
			return false
		}
	}
	return true
}

// HasAnyPermission returns true if at least one of the supplied permissions
// is on the principal.
func (p Principal) HasAnyPermission(opts ...string) bool {
	have := make(map[string]struct{}, len(p.Permissions))
	for _, perm := range p.Permissions {
		have[perm] = struct{}{}
	}
	for _, perm := range opts {
		if _, ok := have[perm]; ok {
			return true
		}
	}
	return false
}

func grantedRoles(role string) map[string]struct{} {
	out := map[string]struct{}{role: {}}
	switch role {
	case "ADMIN":
		out["SUPERVISOR"] = struct{}{}
		out["AGENT"] = struct{}{}
		out["BASIC"] = struct{}{}
	case "SUPERVISOR":
		out["AGENT"] = struct{}{}
		out["BASIC"] = struct{}{}
	case "AGENT":
		out["BASIC"] = struct{}{}
	case "CLIENT_TRADING":
		out["CLIENT_BASIC"] = struct{}{}
	}
	return out
}
