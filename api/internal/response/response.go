package response

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

// Error code constants returned in API error responses.
const (
	CodeValidationError = "VALIDATION_ERROR"
	CodeNotFound        = "NOT_FOUND"
	CodeUnauthorized    = "UNAUTHORIZED"
	CodeForbidden       = "FORBIDDEN"
	CodeConflict        = "CONFLICT"
	CodeInternalError   = "INTERNAL_ERROR"
)

type successBody struct {
	Data any `json:"data"`
}

type listMeta struct {
	Total int `json:"total"`
}

type listBody struct {
	Data any      `json:"data"`
	Meta listMeta `json:"meta"`
}

type errorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type errorBody struct {
	Error errorDetail `json:"error"`
}

type errorDetailWithExtras struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details any    `json:"details"`
}

type errorBodyWithDetails struct {
	Error errorDetailWithExtras `json:"error"`
}

func writeJSON(w http.ResponseWriter, statusCode int, v any) {
	data, err := json.Marshal(v)
	if err != nil {
		slog.Error("failed to marshal JSON response", "error", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":{"code":"INTERNAL_ERROR","message":"internal server error"}}`))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_, _ = w.Write(data)
}

// Success writes a 200 response with {"data": data}.
func Success(w http.ResponseWriter, data any) {
	writeJSON(w, http.StatusOK, successBody{Data: data})
}

// SuccessList writes a 200 response with {"data": data, "meta": {"total": total}}.
func SuccessList(w http.ResponseWriter, data any, total int) {
	writeJSON(w, http.StatusOK, listBody{Data: data, Meta: listMeta{Total: total}})
}

// Error writes an error response with {"error": {"code": code, "message": message}}.
func Error(w http.ResponseWriter, statusCode int, code, message string) {
	writeJSON(w, statusCode, errorBody{
		Error: errorDetail{Code: code, Message: message},
	})
}

// ErrorWithDetails writes an error response that includes a details array.
func ErrorWithDetails(w http.ResponseWriter, statusCode int, code, message string, details any) {
	writeJSON(w, statusCode, errorBodyWithDetails{
		Error: errorDetailWithExtras{Code: code, Message: message, Details: details},
	})
}
