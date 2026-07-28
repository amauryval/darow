package router

import "net/http"

// healthz reports that the process itself is alive, regardless of storage
// state — used by an orchestrator to decide whether to restart the
// container.
//
// @Summary Liveness probe
// @Tags ops
// @Success 200
// @Router /healthz [get]
func (rt *Router) healthz(w http.ResponseWriter, req *http.Request) {
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
func (rt *Router) readyz(w http.ResponseWriter, req *http.Request) {
	if err := rt.ready(req.Context()); err != nil {
		WriteError(w, req, err, http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusOK)
}
