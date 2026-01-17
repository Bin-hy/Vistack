package response

import (
	"net/http"
)

type AppError struct {
	Status int
	Msg    string
	Err    error
}

func (e *AppError) Error() string {
	if e.Err != nil {
		return e.Msg + ": " + e.Err.Error()
	}
	return e.Msg
}

func NewAppError(code int, msg string, err error) *AppError {
	return &AppError{Status: code, Msg: msg, Err: err}
}

// 常用错误类型构造
func NotFound(msg string) *AppError     { return NewAppError(http.StatusNotFound, msg, nil) }
func BadRequest(msg string) *AppError   { return NewAppError(http.StatusBadRequest, msg, nil) }
func Unauthorized(msg string) *AppError { return NewAppError(http.StatusUnauthorized, msg, nil) }
func Forbidden(msg string) *AppError    { return NewAppError(http.StatusForbidden, msg, nil) }
func Conflict(msg string) *AppError     { return NewAppError(http.StatusConflict, msg, nil) }
func Internal(err error) *AppError {
	return NewAppError(http.StatusInternalServerError, "Internal Server Error", err)
}
