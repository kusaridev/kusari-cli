// pkg/auth/errors.go
package auth

import "fmt"

// AuthError represents authentication-related errors
type AuthError struct {
	Code    ErrorCode
	Message string
	Cause   error
}

type ErrorCode int

const (
	ErrUnsupportedProvider ErrorCode = iota
	ErrTokenStorage
	ErrAuthFlow
	ErrTokenExpired
	ErrInvalidToken
	ErrNetworkError
)

func (e *AuthError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("auth error [%d]: %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("auth error [%d]: %s", e.Code, e.Message)
}

func NewAuthError(code ErrorCode, message string) *AuthError {
	return &AuthError{Code: code, Message: message}
}

func NewAuthErrorWithCause(code ErrorCode, message string, cause error) *AuthError {
	return &AuthError{Code: code, Message: message, Cause: cause}
}
