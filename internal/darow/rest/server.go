// Package rest serves darow's routes: the sources, the tables, their rows, tiles and exports, the
// saved filters, and the tables as an OGC API - Features service.
package rest

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/MyAppLibs/go-pkg/apperr"
	"github.com/MyAppLibs/go-pkg/events"
	"github.com/MyAppLibs/go-pkg/repository/resource"

	"github.com/amauryval/darow/internal/darow/engine"
	"github.com/amauryval/darow/internal/darow/filters"
	"github.com/amauryval/darow/internal/darow/settings"
	"github.com/amauryval/darow/internal/darow/sources"
)

// Resources of the events the routes announce: a table's id is its name, a source's its file name.
const (
	TablesResource  = "tables"
	SourcesResource = "sources"
)

// Engine is what the routes read and write of the DuckDB file.
type Engine interface {
	List(ctx context.Context) ([]engine.Table, error)
	Describe(ctx context.Context, name string) (engine.Table, error)
	Extent(ctx context.Context, name string) (*engine.Extent, error)
	Rename(ctx context.Context, name, to string) error
	Drop(ctx context.Context, name string) error
	CreateFromFile(ctx context.Context, name string, load engine.Load) error
	CreateFromQuery(ctx context.Context, name, query string) error
	Layers(ctx context.Context, path string) ([]string, error)
	Rows(ctx context.Context, name string, q engine.RowQuery) (engine.Rows, error)
	Row(ctx context.Context, name string, id int64) (json.RawMessage, error)
	RowWKT(ctx context.Context, name string, id int64, crs string, decimals int) (*string, error)
	InsertRow(ctx context.Context, name string, w engine.RowWrite) (int64, error)
	UpdateRow(ctx context.Context, name string, id int64, w engine.RowWrite) error
	WriteRows(ctx context.Context, name string, batch engine.Batch) ([]int64, error)
	DeleteRow(ctx context.Context, name string, id int64) error
	AddColumn(ctx context.Context, name, column, columnType string) error
	RenameColumn(ctx context.Context, name, column, to string) error
	DropColumn(ctx context.Context, name, column string) error
	Transform(ctx context.Context, name, column, expression, condition string) (int64, error)
	PreviewTransform(ctx context.Context, name, column, expression, condition string) (engine.TransformPreview, error)
	Tile(ctx context.Context, name string, z, x, y int, condition string) ([]byte, error)
	Features(ctx context.Context, name string, q engine.FeatureQuery) (engine.Features, error)
	Feature(ctx context.Context, name string, id int64, decimals int, crs string) (json.RawMessage, error)
	Export(ctx context.Context, name, format, condition, crs, dir string, w io.Writer) error
}

// Deps are what the routes serve: the engine, the sources, the saved filters, the settings, the
// events, and the directory exports are written in.
type Deps struct {
	Engine   Engine
	Sources  *sources.Files
	Filters  *filters.Store
	Settings *settings.Store
	Events   events.Notifier
	TmpDir   string
}

// Server holds what the routes serve.
type Server struct {
	engine        Engine
	sources       *sources.Files
	filters       *filters.Store
	settings      *settings.Store
	events        events.Notifier
	tmpDir        string
	filterHandler *resource.Handler[filters.Filter]
}

// NewServer returns the routes over deps.
func NewServer(deps Deps) *Server {
	return &Server{
		engine: deps.Engine, sources: deps.Sources, filters: deps.Filters, settings: deps.Settings,
		events: deps.Events, tmpDir: deps.TmpDir,
		filterHandler: resource.New[filters.Filter](deps.Filters, "filter", nil),
	}
}

// Register mounts the reads and the administrator's writes, relative to the module's prefix.
func (s *Server) Register(r chi.Router) {
	r.Route("/sources", func(r chi.Router) {
		r.Get("/", s.listSources)
		r.Post("/", s.uploadSource)
		r.Delete("/{name}", s.deleteSource)
		r.Get("/{name}/layers", s.sourceLayers)
		r.Post("/{name}/load", s.loadSource)
	})
	r.Get("/settings", s.getSettings)
	r.Put("/settings", s.putSettings)
	r.Get("/tables", s.listTables)
	r.Post("/tables", s.deriveTable)
	r.Get("/tables/{table}", s.getTable)
	r.Patch("/tables/{table}", s.renameTable)
	r.Delete("/tables/{table}", s.deleteTable)
	r.Get("/tables/{table}/settings", s.getTableSettings)
	r.Put("/tables/{table}/settings", s.putTableSettings)
	r.Get("/tables/{table}/rows", s.listRows)
	r.Get("/tables/{table}/rows/{id}", s.getRow)
	r.Get("/tables/{table}/rows/{id}/wkt", s.getRowWKT)
	r.Get("/tables/{table}/tiles/{z}/{x}/{y}", s.tile)
	r.Get("/tables/{table}/export", s.export)
	resource.RegisterReads(r, "/filters", s.filterHandler)
	r.Route("/ogc", s.registerOGC)
}

// RegisterGuest mounts the writes of the data a guest may make, relative to the module's prefix.
func (s *Server) RegisterGuest(r chi.Router) {
	r.Post("/tables/{table}/rows", s.insertRow)
	r.Patch("/tables/{table}/rows", s.writeRows)
	r.Patch("/tables/{table}/rows/{id}", s.updateRow)
	r.Delete("/tables/{table}/rows/{id}", s.deleteRow)
	r.Post("/tables/{table}/columns", s.addColumn)
	r.Patch("/tables/{table}/columns/{column}", s.renameColumn)
	r.Delete("/tables/{table}/columns/{column}", s.dropColumn)
	r.Post("/tables/{table}/columns/{column}/transform", s.transform)
	r.Post("/tables/{table}/columns/{column}/transform/preview", s.previewTransform)
	resource.RegisterWrites(r, "/filters", s.filterHandler)
}

func (s *Server) changed(r *http.Request, resource, action, id string) {
	s.events.Notify(r.Context(), resource, action, id)
}

func rowID(r *http.Request) (int64, error) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		return 0, apperr.ErrNotFound
	}

	return id, nil
}

// crsParam reads the EPSG code of the srid parameter as a CRS, CRS84 when absent.
func crsParam(r *http.Request) (string, error) {
	srid, err := intParam(r, "srid", 0, maxSRID)

	return engine.CRSOf(srid), err
}

func intParam(r *http.Request, name string, fallback, maximum int) (int, error) {
	raw := r.URL.Query().Get(name)
	if raw == "" {
		return fallback, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < 0 || value > maximum {
		return 0, &apperr.ValidationError{Errors: []apperr.FieldError{{Path: name, Code: apperr.CodeInvalid}}}
	}

	return value, nil
}
