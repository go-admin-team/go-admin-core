package errors

import "net/http"

//go:generate stringer -type ErrorCode -output error_code_string.go

type ErrorCode int32

const (
	OK                  ErrorCode = http.StatusOK
	BadRequest          ErrorCode = http.StatusBadRequest
	Unauthorized        ErrorCode = http.StatusUnauthorized
	Forbidden           ErrorCode = http.StatusForbidden
	NotFound            ErrorCode = http.StatusNotFound
	MethodNotAllowed    ErrorCode = http.StatusMethodNotAllowed
	Timeout             ErrorCode = http.StatusRequestTimeout
	Conflict            ErrorCode = http.StatusConflict
	InternalServerError ErrorCode = http.StatusInternalServerError
)

func (e ErrorCode) Code() int32 {
	return int32(e)
}
