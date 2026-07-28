package router

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"gogeoserver/internal/storage"
)

// tableResolver resolves the {tableId} path param via metadataService.
// Embedded by every handler family that operates on an existing table.
type tableResolver struct {
	metadataService metadataAPI
}

// newTableResolver builds a tableResolver backed by the given metadataAPI port.
func newTableResolver(metadataSvc metadataAPI) tableResolver {
	return tableResolver{metadataService: metadataSvc}
}

// resolveTable fetches the {tableId} path param's metadata (served from
// MetadataService's cache in the common case), writing a 404 and reporting
// ok=false if the table does not exist. Callers must stop handling the
// request when ok is false.
func (tr tableResolver) resolveTable(w http.ResponseWriter, req *http.Request) (meta storage.Metadata, ok bool) {
	tableId := chi.URLParam(req, "tableId")

	meta, err := tr.metadataService.GetMetadata(req.Context(), tableId)
	if err != nil {
		WriteError(w, req, err, http.StatusNotFound)
		return storage.Metadata{}, false
	}
	return meta, true
}
