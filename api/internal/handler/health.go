package handler

import (
	"net/http"

	"github.com/ilyas/mutqin-api/internal/response"
)

// Health returns a handler that responds with the service health status.
func Health() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		response.Success(w, map[string]string{"status": "ok"})
	}
}
