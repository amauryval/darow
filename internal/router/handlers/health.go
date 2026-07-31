package handlers

import (
	"context"
	"net/http"

	"darow/internal/router/response"
)

// OpsHandler serves the liveness/readiness probes used by an orchestrator.
type OpsHandler struct {
	ready func(ctx context.Context) error
}

// NewOpsHandler builds an OpsHandler from the readiness probe it needs.
func NewOpsHandler(ready func(ctx context.Context) error) *OpsHandler {
	return &OpsHandler{ready: ready}
}

// Healthz reports that the process itself is alive, regardless of storage
// state — used by an orchestrator to decide whether to restart the
// container.
//
// @Summary Liveness probe
// @Tags ops
// @Success 200
// @Router /healthz [get]
func (oh *OpsHandler) Healthz(w http.ResponseWriter, req *http.Request) {
	w.WriteHeader(http.StatusOK)
}

// Readyz reports whether the server can currently serve traffic, by
// pinging the underlying storage connection — used by an orchestrator to
// decide whether to route requests to this instance.
//
// @Summary Readiness probe
// @Tags ops
// @Success 200
// @Failure 503 {object} response.ErrorResponse
// @Router /readyz [get]
func (oh *OpsHandler) Readyz(w http.ResponseWriter, req *http.Request) {
	if err := oh.ready(req.Context()); err != nil {
		response.WriteError(w, req, err, http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusOK)
}
