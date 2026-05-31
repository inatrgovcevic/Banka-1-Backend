package service

import "fmt"

type Error struct {
	Status  int
	Code    string
	Title   string
	Message string
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	return e.Message
}

func BadRequest(format string, args ...any) *Error {
	return &Error{Status: 400, Code: "ERR_VALIDATION", Title: "Neispravni podaci", Message: fmt.Sprintf(format, args...)}
}

func NotFound(format string, args ...any) *Error {
	return &Error{Status: 404, Code: "ERR_NOT_FOUND", Title: "Resurs nije pronadjen", Message: fmt.Sprintf(format, args...)}
}

func Conflict(code, title, format string, args ...any) *Error {
	return &Error{Status: 409, Code: code, Title: title, Message: fmt.Sprintf(format, args...)}
}

func Forbidden(format string, args ...any) *Error {
	return &Error{Status: 403, Code: "ERR_FORBIDDEN", Title: "Pristup odbijen", Message: fmt.Sprintf(format, args...)}
}

func Internal(format string, args ...any) *Error {
	return &Error{Status: 500, Code: "ERR_INTERNAL", Title: "Interna greska", Message: fmt.Sprintf(format, args...)}
}
