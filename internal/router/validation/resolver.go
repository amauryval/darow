// Package validation provides the generic request-validation helpers
// shared by every handler family: resolving a table from its path param and
// checking client-supplied column names against a table's real schema.
package validation

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"

	"gogeoserver/internal/router/response"
	"gogeoserver/internal/storage"
)

// MetadataGetter is the port TableResolver needs to look up a table.
type MetadataGetter interface {
	GetMetadata(ctx context.Context, tableId string) (storage.Metadata, error)
}

// TableResolver resolves the {tableId} path param via a MetadataGetter.
// Embedded by every handler family that operates on an existing table.
type TableResolver struct {
	metadataService MetadataGetter
}

// NewTableResolver builds a TableResolver backed by the given MetadataGetter port.
func NewTableResolver(metadataSvc MetadataGetter) TableResolver {
	return TableResolver{metadataService: metadataSvc}
}

// ResolveTable fetches the {tableId} path param's metadata (served from
// MetadataService's cache in the common case), writing a 404 and reporting
// ok=false if the table does not exist. Callers must stop handling the
// request when ok is false.
func (tr TableResolver) ResolveTable(w http.ResponseWriter, req *http.Request) (meta storage.Metadata, ok bool) {
	tableId := chi.URLParam(req, "tableId")

	meta, err := tr.metadataService.GetMetadata(req.Context(), tableId)
	if err != nil {
		response.WriteError(w, req, err, http.StatusNotFound)
		return storage.Metadata{}, false
	}
	return meta, true
}
