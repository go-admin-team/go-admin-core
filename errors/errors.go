// Package errors provides a way to return detailed information
// for an RPC request error. The error is normally JSON encoded.
package errors

import (
	"fmt"
	json "github.com/json-iterator/go"
)

//go:generate protoc -I. --go_out=paths=source_relative:. errors.proto

const (
	Silent       = "0"
	MessageWarn  = "1"
	MessageError = "2"
	Notification = "4"
	Page         = "9"
)

func (e *Error) Error() string {
	b, _ := json.Marshal(e)
	return string(b)
}

// New generates a custom error.
func New(id, domain string, code ErrorCoder) error {
	return &Error{
		ErrorCode:    fmt.Sprintf("C%d", code.Code()),
		ErrorMessage: code.String(),
		ShowType:     MessageError,
		TraceId:      id,
		Domain:       domain,
	}
}

// Parse tries to parse a JSON string into an error. If that
// fails, it will set the given string as the error detail.
func Parse(errStr string) *Error {
	e := new(Error)
	err := json.Unmarshal([]byte(errStr), e)
	if err != nil {
		e.ErrorMessage = errStr
	}
	return e
}

// Equal tries to compare errors
func Equal(err1 error, err2 error) bool {
	verr1, ok1 := err1.(*Error)
	verr2, ok2 := err2.(*Error)

	if ok1 != ok2 {
		return false
	}

	if !ok1 {
		return err1 == err2
	}

	if verr1.ErrorCode != verr2.ErrorCode {
		return false
	}

	return true
}

// FromError try to convert go error to *Error
func FromError(err error) *Error {
	if verr, ok := err.(*Error); ok && verr != nil {
		return verr
	}

	return Parse(err.Error())
}
