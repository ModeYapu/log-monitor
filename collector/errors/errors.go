package errors

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// Error types for better error handling
var (
	// ErrNotFound is returned when a resource is not found
	ErrNotFound = &AppError{Code: "NOT_FOUND", Message: "Resource not found", StatusCode: http.StatusNotFound}

	// ErrUnauthorized is returned when authentication fails
	ErrUnauthorized = &AppError{Code: "UNAUTHORIZED", Message: "Authentication required", StatusCode: http.StatusUnauthorized}

	// ErrForbidden is returned when authorization fails
	ErrForbidden = &AppError{Code: "FORBIDDEN", Message: "Access denied", StatusCode: http.StatusForbidden}

	// ErrBadRequest is returned when the request is invalid
	ErrBadRequest = &AppError{Code: "BAD_REQUEST", Message: "Invalid request", StatusCode: http.StatusBadRequest}

	// ErrConflict is returned when there's a conflict with existing data
	ErrConflict = &AppError{Code: "CONFLICT", Message: "Resource already exists", StatusCode: http.StatusConflict}

	// ErrInternalServer is returned for internal server errors
	ErrInternalServer = &AppError{Code: "INTERNAL_SERVER_ERROR", Message: "Internal server error", StatusCode: http.StatusInternalServerError}

	// ErrDatabase is returned for database errors
	ErrDatabase = &AppError{Code: "DATABASE_ERROR", Message: "Database operation failed", StatusCode: http.StatusInternalServerError}

	// ErrValidation is returned when validation fails
	ErrValidation = &AppError{Code: "VALIDATION_ERROR", Message: "Validation failed", StatusCode: http.StatusBadRequest}
)

// AppError represents an application error with HTTP status code
type AppError struct {
	Code       string `json:"code"`
	Message    string `json:"message"`
	StatusCode int    `json:"-"`
	Details    string `json:"details,omitempty"`
	Field      string `json:"field,omitempty"`
}

// Error implements the error interface
func (e *AppError) Error() string {
	if e.Details != "" {
		return fmt.Sprintf("%s: %s (%s)", e.Code, e.Message, e.Details)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// NewNotFoundError creates a new NOT_FOUND error
func NewNotFoundError(resource string) *AppError {
	return &AppError{
		Code:       "NOT_FOUND",
		Message:    fmt.Sprintf("%s not found", resource),
		StatusCode: http.StatusNotFound,
	}
}

// NewUnauthorizedError creates a new UNAUTHORIZED error
func NewUnauthorizedError(message string) *AppError {
	if message == "" {
		message = "Authentication required"
	}
	return &AppError{
		Code:       "UNAUTHORIZED",
		Message:    message,
		StatusCode: http.StatusUnauthorized,
	}
}

// NewForbiddenError creates a new FORBIDDEN error
func NewForbiddenError(message string) *AppError {
	if message == "" {
		message = "Access denied"
	}
	return &AppError{
		Code:       "FORBIDDEN",
		Message:    message,
		StatusCode: http.StatusForbidden,
	}
}

// NewBadRequestError creates a new BAD_REQUEST error
func NewBadRequestError(message string) *AppError {
	if message == "" {
		message = "Invalid request"
	}
	return &AppError{
		Code:       "BAD_REQUEST",
		Message:    message,
		StatusCode: http.StatusBadRequest,
	}
}

// NewConflictError creates a new CONFLICT error
func NewConflictError(resource string) *AppError {
	return &AppError{
		Code:       "CONFLICT",
		Message:    fmt.Sprintf("%s already exists", resource),
		StatusCode: http.StatusConflict,
	}
}

// NewValidationError creates a new VALIDATION_ERROR error
func NewValidationError(field string, message string) *AppError {
	if message == "" {
		message = "Validation failed"
	}
	return &AppError{
		Code:       "VALIDATION_ERROR",
		Message:    message,
		StatusCode: http.StatusBadRequest,
		Field:      field,
	}
}

// NewInternalError creates a new INTERNAL_SERVER_ERROR
func NewInternalError(message string) *AppError {
	if message == "" {
		message = "Internal server error"
	}
	return &AppError{
		Code:       "INTERNAL_SERVER_ERROR",
		Message:    message,
		StatusCode: http.StatusInternalServerError,
	}
}

// NewDatabaseError creates a new DATABASE_ERROR
func NewDatabaseError(message string) *AppError {
	if message == "" {
		message = "Database operation failed"
	}
	return &AppError{
		Code:       "DATABASE_ERROR",
		Message:    message,
		StatusCode: http.StatusInternalServerError,
	}
}

// WrapError wraps an error with additional context
func WrapError(err error, message string) *AppError {
	if appErr, ok := err.(*AppError); ok {
		return &AppError{
			Code:       appErr.Code,
			Message:    message + ": " + appErr.Message,
			StatusCode: appErr.StatusCode,
		}
	}
	return &AppError{
		Code:       "INTERNAL_SERVER_ERROR",
		Message:    message + ": " + err.Error(),
		StatusCode: http.StatusInternalServerError,
	}
}

// IsAppError checks if an error is an AppError
func IsAppError(err error) bool {
	_, ok := err.(*AppError)
	return ok
}

// GetStatusCode returns the HTTP status code for an error
func GetStatusCode(err error) int {
	if appErr, ok := err.(*AppError); ok {
		return appErr.StatusCode
	}
	return http.StatusInternalServerError
}

// ErrorResponse represents a standardized error response
type ErrorResponse struct {
	Error ErrorDetail `json:"error"`
}

// ErrorDetail represents error details
type ErrorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details string `json:"details,omitempty"`
	Field   string `json:"field,omitempty"`
}

// WriteErrorResponse writes an error response to HTTP response writer
func WriteErrorResponse(w http.ResponseWriter, err error) {
	statusCode := GetStatusCode(err)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	var response ErrorResponse
	if appErr, ok := err.(*AppError); ok {
		response = ErrorResponse{
			Error: ErrorDetail{
				Code:    appErr.Code,
				Message: appErr.Message,
				Details: appErr.Details,
				Field:   appErr.Field,
			},
		}
	} else {
		response = ErrorResponse{
			Error: ErrorDetail{
				Code:    "INTERNAL_SERVER_ERROR",
				Message: err.Error(),
			},
		}
	}

	json.NewEncoder(w).Encode(response)
}

// WriteJSONResponse writes a JSON response or error
func WriteJSONResponse(w http.ResponseWriter, data interface{}, err error) {
	if err != nil {
		WriteErrorResponse(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(data)
}