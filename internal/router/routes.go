package router

import (
	"context"
	"io/fs"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	httpSwagger "github.com/swaggo/http-swagger"

	_ "darow/docs"
	"darow/internal/config"
	"darow/internal/core"
	"darow/internal/router/handlers"
)

// Router wires the HTTP layer to the service layer. Each family of
// endpoints (tables, rows, geo, sources, export, ops) is served by its own
// handler struct, holding only the service ports it actually needs — never
// the concrete service types.
type Router struct {
	tables   *handlers.TablesHandler
	rows     *handlers.RowsHandler
	geo      *handlers.GeoHandler
	features *handlers.FeaturesHandler
	sources  *handlers.SourcesHandler
	filters  *handlers.FiltersHandler
	export   *handlers.ExportHandler
	ops      *handlers.OpsHandler
	apiCfg   config.APIConfig
	uiAssets fs.FS
}

// NewRouter builds a Router serving every service in svcs, honoring
// apiCfg's row pagination limits. uiAssets is the admin UI's
// built static files, served under /ui. ready backs /readyz — it should
// ping the underlying storage connection.
func NewRouter(
	svcs *core.Services, apiCfg config.APIConfig, uiAssets fs.FS, ready func(ctx context.Context) error,
) *Router {
	return &Router{
		tables:   handlers.NewTablesHandler(svcs.Metadata, svcs.Schema, svcs.Derive, svcs.Row, apiCfg),
		rows:     handlers.NewRowsHandler(svcs.Metadata, svcs.Schema, svcs.Row, apiCfg),
		geo:      handlers.NewGeoHandler(svcs.Metadata, svcs.Schema, svcs.Geo, apiCfg),
		features: handlers.NewFeaturesHandler(svcs.Metadata, svcs.Schema, svcs.Geo, apiCfg),
		sources:  handlers.NewSourcesHandler(svcs.Source, apiCfg),
		filters:  handlers.NewFiltersHandler(svcs.Metadata, svcs.Filter),
		export:   handlers.NewExportHandler(svcs.Metadata, svcs.Export),
		ops:      handlers.NewOpsHandler(ready),
		apiCfg:   apiCfg,
		uiAssets: uiAssets,
	}
}

// Setup builds the chi router with every route from spec/back/spec.md section 6.1.
func (rt *Router) Setup() http.Handler {
	r := chi.NewRouter()

	r.NotFound(notFoundJSON)
	r.MethodNotAllowed(methodNotAllowedJSON)

	r.Use(middleware.RequestID)
	r.Use(slogLogger)
	r.Use(recoverJSON)
	r.Use(maxBodyBytes(rt.apiCfg.MaxBodyBytes))

	r.Get("/healthz", rt.ops.Healthz)
	r.Get("/readyz", rt.ops.Readyz)

	r.Get("/swagger/*", httpSwagger.WrapHandler)

	r.Get("/", func(w http.ResponseWriter, req *http.Request) {
		http.Redirect(w, req, "/ui/", http.StatusFound)
	})
	r.Handle("/ui", http.RedirectHandler("/ui/", http.StatusFound))
	r.Handle("/ui/*", mountUI(rt.uiAssets))

	r.Route("/api/v1", func(v1 chi.Router) {
		// middleware.Timeout is scoped to this Group rather than applied at
		// the very top of Setup, so it never wraps exportTable below — that
		// route is registered outside the Group precisely to escape it.
		// Everything else keeps the same bounded-request budget as before.
		v1.Group(func(v1 chi.Router) {
			if rt.apiCfg.RequestTimeoutSec > 0 {
				v1.Use(middleware.Timeout(time.Duration(rt.apiCfg.RequestTimeoutSec) * time.Second))
			}

			v1.Route("/sources", func(sub chi.Router) {
				sub.Get("/", rt.sources.ListSources)
				sub.Post("/refresh", rt.sources.RefreshSources)
				// Overrides the global body cap: an upload is a whole source file
				// (up to sources.maxSizeBytes), not a JSON payload. The extra 64KiB
				// covers multipart framing (boundary, headers) around the file part
				// itself, so a file right at the limit isn't cut off by framing
				// overhead before SourceService.Upload's own oversize check runs.
				sub.With(maxBodyBytes(rt.sources.MaxUploadBytes()+64*1024)).Post("/upload", rt.sources.UploadSource)
				sub.Post("/{name}/load", rt.sources.LoadSource)
				sub.Get("/{name}/metadata", rt.sources.GetSourceMetadata)
				sub.Delete("/{name}", rt.sources.DeleteSource)
			})

			v1.Route("/tables", func(sub chi.Router) {
				sub.Get("/", rt.tables.ListTables)

				sub.Route("/{tableId}", func(sub chi.Router) {
					sub.Get("/", rt.tables.GetTableMetadata)
					sub.Patch("/", rt.tables.RenameTable)
					sub.Delete("/", rt.tables.DeleteTable)

					sub.Get("/schema", rt.tables.GetSchema)
					sub.Post("/columns", rt.tables.AddColumn)
					sub.Post("/derive", rt.tables.DeriveTable)
					sub.Post("/filter", rt.tables.FilterRows)

					sub.Route("/filters", func(sub chi.Router) {
						sub.Get("/", rt.filters.ListFilters)
						sub.Post("/", rt.filters.CreateFilter)
						sub.Patch("/{filterId}", rt.filters.UpdateFilter)
						sub.Delete("/{filterId}", rt.filters.DeleteFilter)
					})
					sub.Get("/columns/{col}/distinct", rt.tables.GetDistinctValues)

					sub.Route("/rows", func(sub chi.Router) {
						sub.Get("/", rt.rows.ListRows)
						sub.Post("/", rt.rows.AddRows)
						sub.Post("/batch", rt.rows.GetRowsByPkey)
						sub.Route("/{pkey}", func(sub chi.Router) {
							sub.Get("/", rt.rows.GetRow)
							sub.Patch("/", rt.rows.UpdateRow)
							sub.Delete("/", rt.rows.DeleteRow)
						})
					})

					sub.Get("/tiles/{z}/{x}/{y}", rt.geo.GetTile)
					sub.Get("/tiles/{z}/{x}/{y}/features", rt.geo.GetTileFeatures)
					sub.Get("/geometry-types", rt.geo.GetGeometryTypes)
					sub.Post("/bbox", rt.geo.FilterByBbox)
					sub.Get("/filtered-extent", rt.geo.GetFilteredExtent)
				})
			})

			v1.Route("/ogc", func(sub chi.Router) {
				sub.Get("/", rt.features.LandingPage)
				sub.Get("/conformance", rt.features.Conformance)
				sub.Route("/collections", func(sub chi.Router) {
					sub.Get("/", rt.features.ListCollections)
					sub.Route("/{tableId}", func(sub chi.Router) {
						sub.Get("/", rt.features.GetCollection)
						sub.Route("/items", func(sub chi.Router) {
							sub.Get("/", rt.features.GetItems)
							sub.Get("/{featureId}", rt.features.GetItem)
						})
					})
				})
			})
		})

		// ExportTable streams a table in bounded batches and can
		// legitimately run past any normal request's time budget on a large
		// table — middleware.Timeout would otherwise cancel it mid-stream,
		// truncating an already-200'd response.
		v1.Get("/tables/{tableId}/export", rt.export.ExportTable)
	})

	return r
}
