// Package httpapi contains the JSON and download helpers shared by the REST handlers of the master.
package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const maxBodyBytes = 1 << 20

// Error is an error whose message is safe to show to API clients.
type Error struct {
	Status  int
	Message string
}

func (e *Error) Error() string { return e.Message }

// Errorf returns an *Error with the given HTTP status.
func Errorf(status int, format string, args ...any) error {
	return &Error{Status: status, Message: fmt.Sprintf(format, args...)}
}

// grpcStatus maps errors returned by agents to HTTP statuses. Their messages are
// shown to administrators, which helps diagnosing problems on a node.
var grpcStatus = map[codes.Code]int{
	codes.InvalidArgument:    http.StatusBadRequest,
	codes.NotFound:           http.StatusNotFound,
	codes.AlreadyExists:      http.StatusConflict,
	codes.FailedPrecondition: http.StatusConflict,
	codes.PermissionDenied:   http.StatusForbidden,
	codes.ResourceExhausted:  http.StatusRequestEntityTooLarge,
	codes.Internal:           http.StatusBadGateway,
	codes.Unavailable:        http.StatusBadGateway,
	codes.DeadlineExceeded:   http.StatusGatewayTimeout,
}

// WriteJSON writes v as JSON with the given status.
func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if v != nil {
		_ = json.NewEncoder(w).Encode(v)
	}
}

// WriteError writes err as {"error": "..."}. Unexpected errors are logged and hidden.
func WriteError(w http.ResponseWriter, r *http.Request, err error) {
	var apiErr *Error
	if errors.As(err, &apiErr) {
		WriteJSON(w, apiErr.Status, map[string]string{"error": apiErr.Message})
		return
	}
	if st, ok := status.FromError(err); ok {
		if code, known := grpcStatus[st.Code()]; known {
			WriteJSON(w, code, map[string]string{"error": st.Message()})
			return
		}
	}
	slog.Error("Request failed", "method", r.Method, "path", r.URL.Path, "err", err)
	WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
}

// ReadJSON decodes a size limited request body into v and rejects unknown fields.
func ReadJSON(w http.ResponseWriter, r *http.Request, v any) error {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBodyBytes))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return Errorf(http.StatusBadRequest, "invalid request body: %v", err)
	}
	return nil
}
