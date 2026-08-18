package handlers

import (
	"context"
	"net/http"

	"darow/internal/router/response"
	"darow/internal/storage"
)

// OpsHandler serves the liveness/readiness probes and the diagnostics
// snapshot used by an orchestrator or an operator.
type OpsHandler struct {
	ready        func(ctx context.Context) error
	diagnostics  diagnosticsAPI
	maxSizeBytes int64
}

// NewOpsHandler builds an OpsHandler from the readiness probe, the
// diagnostics service, and the configured database size cap
// (duckdb.maxSizeBytes, 0 meaning unlimited).
func NewOpsHandler(ready func(ctx context.Context) error, diagnostics diagnosticsAPI, maxSizeBytes int64) *OpsHandler {
	return &OpsHandler{ready: ready, diagnostics: diagnostics, maxSizeBytes: maxSizeBytes}
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

// diagnosticsDTO is the JSON representation of a Diagnostics snapshot.
// MaxSizeBytes is duckdb.maxSizeBytes from config, not part of
// storage.Diagnostics itself — 0 means unlimited.
type diagnosticsDTO struct {
	DatabaseSizeBytes   int64    `json:"databaseSizeBytes"`
	MemoryUsageBytes    int64    `json:"memoryUsageBytes"`
	TemporaryFilesBytes int64    `json:"temporaryFilesBytes"`
	Extensions          []string `json:"extensions"`
	MaxSizeBytes        int64    `json:"maxSizeBytes"`
}

func toDiagnosticsDTO(d storage.Diagnostics, maxSizeBytes int64) diagnosticsDTO {
	return diagnosticsDTO{
		DatabaseSizeBytes:   d.DatabaseSizeBytes,
		MemoryUsageBytes:    d.MemoryUsageBytes,
		TemporaryFilesBytes: d.TemporaryFilesBytes,
		Extensions:          d.Extensions,
		MaxSizeBytes:        maxSizeBytes,
	}
}

// Diagnostics reports the storage engine's own operating state — on-disk
// size, buffer-manager memory usage, bytes spilled to disk, loaded
// extensions — for an operator to tell "up" from "healthy".
//
// @Summary Storage engine diagnostics
// @Tags ops
// @Produce json
// @Success 200 {object} diagnosticsDTO
// @Failure 500 {object} response.ErrorResponse
// @Router /diagnostics [get]
func (oh *OpsHandler) Diagnostics(w http.ResponseWriter, req *http.Request) {
	d, err := oh.diagnostics.Diagnostics(req.Context())
	if err != nil {
		response.WriteServiceError(w, req, err)
		return
	}
	response.WriteJSON(w, http.StatusOK, toDiagnosticsDTO(d, oh.maxSizeBytes))
}
