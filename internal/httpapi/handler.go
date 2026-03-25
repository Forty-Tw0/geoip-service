package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"

	"geoip-service/internal/authorize"
)

type checkRequest struct {
	IPAddress        string   `json:"ip_address"`
	AllowedCountries []string `json:"allowed_countries"`
}

type CheckFunc func(context.Context, string, []string) (authorize.Decision, error)

type UpdateFunc func(context.Context) error

type handler struct {
	logger *slog.Logger
	check  CheckFunc
	update UpdateFunc
}

func NewHandler(
	logger *slog.Logger,
	check CheckFunc,
	update UpdateFunc,
) http.Handler {
	h := handler{
		logger: logger,
		check:  check,
		update: update,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", h.health)
	mux.HandleFunc("POST /update", h.updateDatabase)
	mux.HandleFunc("POST /v1/check", h.checkAuthorization)
	return mux
}

func (h *handler) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *handler) checkAuthorization(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	req, err := decodeCheckRequest(r)
	if err != nil {
		writeRequestError(w, err)
		return
	}

	decision, err := h.check(r.Context(), req.IPAddress, req.AllowedCountries)
	if err != nil {
		writeServiceError(h.logger, w, err)
		return
	}

	writeJSON(w, http.StatusOK, decision)
}

func (h *handler) updateDatabase(w http.ResponseWriter, r *http.Request) {
	if h.update == nil {
		writeError(w, http.StatusServiceUnavailable, "maxmind credentials are not configured")
		return
	}

	if err := h.update(r.Context()); err != nil {
		h.logger.Error("database update failed", "error", err)
		writeError(w, http.StatusInternalServerError, "database update failed")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

func decodeCheckRequest(r *http.Request) (checkRequest, error) {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	var req checkRequest
	if err := decoder.Decode(&req); err != nil {
		return checkRequest{}, err
	}

	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return checkRequest{}, errors.New("request body must contain a single JSON object")
	}

	return req, nil
}

func writeRequestError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, io.EOF):
		writeError(w, http.StatusBadRequest, "request body is required")
	default:
		writeError(w, http.StatusBadRequest, "invalid request body")
	}
}

func writeServiceError(logger *slog.Logger, w http.ResponseWriter, err error) {
	kind, message := authorize.ClassifyError(err)

	switch kind {
	case authorize.ErrorKindInvalidArgument:
		writeError(w, http.StatusBadRequest, message)
	case authorize.ErrorKindNotFound:
		writeError(w, http.StatusNotFound, message)
	default:
		logger.Error("authorization check failed", "error", err)
		writeError(w, http.StatusInternalServerError, message)
	}
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
