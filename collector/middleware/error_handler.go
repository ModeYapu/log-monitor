package middleware

import (
	"log/slog"
	"net/http"

	"github.com/logmonitor/collector/errors"
)

// ErrorHandler creates middleware for consistent error handling
func ErrorHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Catch panics and convert them to errors
		defer func() {
			if recovered := recover(); recovered != nil {
				slog.Error("Panic recovered in HTTP handler", "panic", recovered)
				errors.WriteErrorResponse(w, errors.NewInternalError("Internal server error"))
			}
		}()

		// Create a custom response writer to capture errors
		wrapped := &errorHandlerResponseWriter{
			ResponseWriter: w,
			Request:       r,
		}

		// Call the next handler
		next.ServeHTTP(wrapped, r)

		// If there was an error, write the error response
		if wrapped.err != nil {
			errors.WriteErrorResponse(w, wrapped.err)
		}
	})
}

// errorHandlerResponseWriter wraps http.ResponseWriter to capture errors
type errorHandlerResponseWriter struct {
	http.ResponseWriter
	Request    *http.Request
	err        error
	statusCode int
}

// Write implements the http.ResponseWriter interface
func (w *errorHandlerResponseWriter) Write(b []byte) (int, error) {
	if w.err != nil {
		return 0, w.err
	}
	return w.ResponseWriter.Write(b)
}

// WriteHeader implements the http.ResponseWriter interface
func (w *errorHandlerResponseWriter) WriteHeader(statusCode int) {
	if w.statusCode != 0 {
		return
	}
	w.statusCode = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}

// HandleError is a helper function for handlers to return errors
func HandleError(w http.ResponseWriter, r *http.Request, err error) {
	if err == nil {
		return
	}
	slog.Error("Handler error", "path", r.URL.Path, "method", r.Method, "error", err)
	errors.WriteErrorResponse(w, err)
}

// RespondJSON is a helper function for consistent JSON responses
func RespondJSON(w http.ResponseWriter, r *http.Request, data interface{}, err error) {
	if err != nil {
		HandleError(w, r, err)
		return
	}
	errors.WriteJSONResponse(w, data, nil)
}
