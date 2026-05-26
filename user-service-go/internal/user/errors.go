package user

import "errors"

var (
	ErrBadRequest      = errors.New("bad request")
	ErrInvalidLogin    = errors.New("invalid credentials")
	ErrNotFound        = errors.New("not found")
	ErrForbidden       = errors.New("forbidden")
	ErrInactiveAccount = errors.New("inactive account")
	ErrLockedAccount   = errors.New("account locked")
	ErrInvalidToken    = errors.New("invalid token")
	ErrDuplicate       = errors.New("duplicate resource")
	ErrUnsupportedJMBG = errors.New("jmbg plaintext lookup is unavailable after encryption migration")
)
