package handlers

import (
	"context"
	"net/http"

	"darow/internal/router/response"
	"darow/internal/storage"
)

// OpsHandler serves the liveness/readiness probes, the diagnostics snapshot
// used by an orchestrator or an operator, and the build version.
type OpsHandler struct {
	ready        func(ctx context.Context) error
	diagnostics  diagnosticsAPI
	maxSizeBytes int64
	buildVersion string
	buildCommit  string
	buildDate    string
}

// NewOpsHandler builds an OpsHandler from the readiness probe, the
// diagnostics service, the configured database size cap (duckdb.maxSizeBytes,
// 0 meaning unlimited), and the build version/commit/date (internal/version).
func NewOpsHandler(ready func(ctx context.Context) error, diagnostics diagnosticsAPI, maxSizeBytes int64, buildVersion, buildCommit, buildDate string) *OpsHandler {
	return &OpsHandler{ready: ready, diagnostics: diagnostics, maxSizeBytes: maxSizeBytes, buildVersion: buildVersion, buildCommit: buildCommit, buildDate: buildDate}
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

// versionDTO is the JSON representation of the running build's identity.
type versionDTO struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	BuildDate string `json:"buildDate"`
}

// Version reports the build this process was started from — the git
// describe output, the short commit SHA and the build timestamp, all baked
// in at build time.
//
// @Summary Build version
// @Tags ops
// @Produce json
// @Success 200 {object} versionDTO
// @Router /version [get]
func (oh *OpsHandler) Version(w http.ResponseWriter, req *http.Request) {
	response.WriteJSON(w, http.StatusOK, versionDTO{Version: oh.buildVersion, Commit: oh.buildCommit, BuildDate: oh.buildDate})
}
