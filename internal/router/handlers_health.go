package router

import (
	"context"
	"net/http"
)

// opsHandler serves the liveness/readiness probes used by an orchestrator.
type opsHandler struct {
	ready func(ctx context.Context) error
}

// newOpsHandler builds an opsHandler from the readiness probe it needs.
func newOpsHandler(ready func(ctx context.Context) error) *opsHandler {
	return &opsHandler{ready: ready}
}

// healthz reports that the process itself is alive, regardless of storage
// state — used by an orchestrator to decide whether to restart the
// container.
//
// @Summary Liveness probe
// @Tags ops
// @Success 200
// @Router /healthz [get]
func (oh *opsHandler) healthz(w http.ResponseWriter, req *http.Request) {
	w.WriteHeader(http.StatusOK)
}

// readyz reports whether the server can currently serve traffic, by
// pinging the underlying storage connection — used by an orchestrator to
// decide whether to route requests to this instance.
//
// @Summary Readiness probe
// @Tags ops
// @Success 200
// @Failure 503 {object} ErrorResponse
// @Router /readyz [get]
func (oh *opsHandler) readyz(w http.ResponseWriter, req *http.Request) {
	if err := oh.ready(req.Context()); err != nil {
		WriteError(w, req, err, http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusOK)
}
