package rstmdb

import (
	"errors"
	"fmt"
)

// ErrorCode represents SCREAMING_SNAKE_CASE error codes from the server.
type ErrorCode string

const (
	ErrCodeUnsupportedProtocol         ErrorCode = "UNSUPPORTED_PROTOCOL"
	ErrCodeBadRequest                  ErrorCode = "BAD_REQUEST"
	ErrCodeUnauthorized                ErrorCode = "UNAUTHORIZED"
	ErrCodeAuthFailed                  ErrorCode = "AUTH_FAILED"
	ErrCodeNotFound                    ErrorCode = "NOT_FOUND"
	ErrCodeMachineNotFound             ErrorCode = "MACHINE_NOT_FOUND"
	ErrCodeMachineVersionExists        ErrorCode = "MACHINE_VERSION_EXISTS"
	ErrCodeMachineVersionLimitExceeded ErrorCode = "MACHINE_VERSION_LIMIT_EXCEEDED"
	ErrCodeInstanceNotFound            ErrorCode = "INSTANCE_NOT_FOUND"
	ErrCodeInstanceExists              ErrorCode = "INSTANCE_EXISTS"
	ErrCodeInvalidTransition           ErrorCode = "INVALID_TRANSITION"
	ErrCodeGuardFailed                 ErrorCode = "GUARD_FAILED"
	ErrCodeGuardUnknown                ErrorCode = "GUARD_UNKNOWN"
	ErrCodeConflict                    ErrorCode = "CONFLICT"
	ErrCodeIdempotencyConflict         ErrorCode = "IDEMPOTENCY_CONFLICT"
	ErrCodeReadOnlyMode                ErrorCode = "READ_ONLY_MODE"
	ErrCodeReplicationTimeout          ErrorCode = "REPLICATION_TIMEOUT"
	ErrCodeReplicationError            ErrorCode = "REPLICATION_ERROR"
	ErrCodeWALIOError                  ErrorCode = "WAL_IO_ERROR"
	ErrCodeInternalError               ErrorCode = "INTERNAL_ERROR"
	ErrCodeRateLimited                 ErrorCode = "RATE_LIMITED"
)

// Error represents an rstmdb application-level error.
type Error struct {
	Code      ErrorCode      `json:"code"`
	Message   string         `json:"message"`
	Retryable bool           `json:"retryable"`
	Details   map[string]any `json:"details,omitempty"`
}

func (e *Error) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("rstmdb: %s: %s", e.Code, e.Message)
	}
	return fmt.Sprintf("rstmdb: %s", e.Code)
}

// Protocol-level errors (frame parsing failures).
var (
	ErrInvalidMagic       = errors.New("rstmdb: invalid frame magic (expected RCPX)")
	ErrUnsupportedVersion = errors.New("rstmdb: unsupported protocol version")
	ErrFrameTooLarge      = errors.New("rstmdb: frame exceeds MAX_PAYLOAD_SIZE (16 MiB)")
	ErrCRCMismatch        = errors.New("rstmdb: CRC32C checksum mismatch")
	ErrInvalidFlags       = errors.New("rstmdb: invalid frame flags")
	ErrConnectionClosed   = errors.New("rstmdb: connection closed")
	ErrTimeout            = errors.New("rstmdb: request timeout")
)

// IsNotFound reports whether err is an rstmdb NOT_FOUND error.
func IsNotFound(err error) bool {
	return hasCode(err, ErrCodeNotFound)
}

// IsInstanceNotFound reports whether err is an rstmdb INSTANCE_NOT_FOUND error.
func IsInstanceNotFound(err error) bool {
	return hasCode(err, ErrCodeInstanceNotFound)
}

// IsMachineNotFound reports whether err is an rstmdb MACHINE_NOT_FOUND error.
func IsMachineNotFound(err error) bool {
	return hasCode(err, ErrCodeMachineNotFound)
}

// IsInvalidTransition reports whether err is an rstmdb INVALID_TRANSITION error.
func IsInvalidTransition(err error) bool {
	return hasCode(err, ErrCodeInvalidTransition)
}

// IsGuardFailed reports whether err is an rstmdb GUARD_FAILED error.
func IsGuardFailed(err error) bool {
	return hasCode(err, ErrCodeGuardFailed)
}

// IsGuardUnknown reports whether err is an rstmdb GUARD_UNKNOWN error.
func IsGuardUnknown(err error) bool {
	return hasCode(err, ErrCodeGuardUnknown)
}

// IsConflict reports whether err is an rstmdb CONFLICT error.
func IsConflict(err error) bool {
	return hasCode(err, ErrCodeConflict)
}

// IsIdempotencyConflict reports whether err is an rstmdb IDEMPOTENCY_CONFLICT error.
func IsIdempotencyConflict(err error) bool {
	return hasCode(err, ErrCodeIdempotencyConflict)
}

// IsReadOnlyMode reports whether err is an rstmdb READ_ONLY_MODE error.
func IsReadOnlyMode(err error) bool {
	return hasCode(err, ErrCodeReadOnlyMode)
}

// IsReplicationTimeout reports whether err is an rstmdb REPLICATION_TIMEOUT error.
func IsReplicationTimeout(err error) bool {
	return hasCode(err, ErrCodeReplicationTimeout)
}

// IsReplicationError reports whether err is an rstmdb REPLICATION_ERROR error.
func IsReplicationError(err error) bool {
	return hasCode(err, ErrCodeReplicationError)
}

// IsRetryable reports whether the error is safe to retry.
func IsRetryable(err error) bool {
	var e *Error
	if errors.As(err, &e) {
		return e.Retryable
	}
	return false
}

func hasCode(err error, code ErrorCode) bool {
	var e *Error
	if errors.As(err, &e) {
		return e.Code == code
	}
	return false
}
