package apperror

import "fmt"

type Code string

const (
	CodeInternal         Code = "internal"
	CodeInvalidRequest   Code = "invalid_request"
	CodeNotFound         Code = "not_found"
	CodeNotReady         Code = "not_ready"
	CodeMethodNotAllowed Code = "method_not_allowed"
	CodeUnauthorized     Code = "unauthorized"
	CodeForbidden        Code = "forbidden"
	CodeConflict         Code = "conflict"
	CodeQuotaExhausted   Code = "quota_exhausted"
)

type Error struct {
	Code       Code
	Message    string
	HTTPStatus int
	Cause      error
}

func New(code Code, message string, httpStatus int) *Error {
	return &Error{
		Code:       code,
		Message:    message,
		HTTPStatus: httpStatus,
	}
}

func Wrap(code Code, message string, httpStatus int, cause error) *Error {
	return &Error{
		Code:       code,
		Message:    message,
		HTTPStatus: httpStatus,
		Cause:      cause,
	}
}

func (err *Error) Error() string {
	if err.Cause == nil {
		return fmt.Sprintf("%s: %s", err.Code, err.Message)
	}

	return fmt.Sprintf("%s: %s: %v", err.Code, err.Message, err.Cause)
}

func (err *Error) Unwrap() error {
	return err.Cause
}
