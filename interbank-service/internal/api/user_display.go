package api

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)

// ---------------------------------------------------------------------------
// Interfaces
// ---------------------------------------------------------------------------

// UserResolver resolves a user by type+id to a display name.
type UserResolver interface {
	// ResolveUser looks up display info for a user in our bank.
	// userType is "CLIENT" or "EMPLOYEE"; id is the numeric user id.
	// Returns (nil, nil) or (nil, error) — never both non-nil.
	ResolveUser(ctx context.Context, userType string, id int64) (*UserDisplayInfo, error)
}

// UserDisplayInfo is returned by ResolveUser.
type UserDisplayInfo struct {
	DisplayName string
}

// ---------------------------------------------------------------------------
// UserDisplayHandler
// ---------------------------------------------------------------------------

// UserDisplayHandler handles GET /interbank/user/{rn}/{id} and GET /user/{rn}/{id}.
// Corresponds to Java UserDisplayController.
type UserDisplayHandler struct {
	myRouting      int
	myDisplayName  string
	resolver       UserResolver
	log            *slog.Logger
}

// UserInformationDto is the wire response per Tim 2 §3.7.
type UserInformationDto struct {
	BankDisplayName string `json:"bankDisplayName"`
	DisplayName     string `json:"displayName"`
}

// NewUserDisplayHandler constructs the handler.
func NewUserDisplayHandler(myRouting int, myDisplayName string, resolver UserResolver, log *slog.Logger) *UserDisplayHandler {
	if log == nil {
		log = slog.Default()
	}
	return &UserDisplayHandler{
		myRouting:     myRouting,
		myDisplayName: myDisplayName,
		resolver:      resolver,
		log:           log,
	}
}

// Get handles GET /interbank/user/{rn}/{id} and GET /user/{rn}/{id}.
func (h *UserDisplayHandler) Get(w http.ResponseWriter, r *http.Request) {
	rnStr := chi.URLParam(r, "rn")
	id := chi.URLParam(r, "id")

	rn, err := strconv.Atoi(rnStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid routing number: "+rnStr)
		return
	}
	// rn must be our routing number — we only know about our own users.
	if rn != h.myRouting {
		writeError(w, http.StatusNotFound, fmt.Sprintf("user %d/%s not in this bank", rn, id))
		return
	}
	// Parse prefix.
	if len(id) < 3 {
		writeError(w, http.StatusBadRequest, "invalid user id format: "+id)
		return
	}
	var userType string
	var numericPart string
	switch {
	case len(id) > 2 && id[:2] == "C-":
		userType = "CLIENT"
		numericPart = id[2:]
	case len(id) > 2 && id[:2] == "E-":
		userType = "EMPLOYEE"
		numericPart = id[2:]
	default:
		writeError(w, http.StatusBadRequest, "user id must start with C- or E-: "+id)
		return
	}
	numericID, err := strconv.ParseInt(numericPart, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "user id numeric part not valid: "+numericPart)
		return
	}

	info, err := h.resolver.ResolveUser(r.Context(), userType, numericID)
	if err != nil {
		h.log.WarnContext(r.Context(), "user-display: resolve failed", "id", id, "err", err)
		writeError(w, http.StatusNotFound, "user "+id+" not found")
		return
	}
	if info == nil {
		writeError(w, http.StatusNotFound, "user "+id+" not found")
		return
	}

	writeJSON(w, http.StatusOK, UserInformationDto{
		BankDisplayName: h.myDisplayName,
		DisplayName:     info.DisplayName,
	})
}
