package router

import (
	"context"
	"io/fs"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	httpSwagger "github.com/swaggo/http-swagger"

	_ "gogeoserver/docs"
	"gogeoserver/internal/config"
	"gogeoserver/internal/service"
)

// Router wires the HTTP layer to the service layer. Each family of
// endpoints (tables, rows, geo, sources, export, ops) is served by its own
// handler struct, holding only the service ports (see ports.go) and config
// it actually needs — never the concrete service types.
type Router struct {
	tables   *tablesHandler
	rows     *rowsHandler
	geo      *geoHandler
	sources  *sourcesHandler
	export   *exportHandler
	ops      *opsHandler
	apiCfg   config.APIConfig
	uiAssets fs.FS
}

// NewRouter builds a Router serving every service above, honoring
// apiCfg's row pagination limits (règle 18). uiAssets is the admin UI's
// built static files, served under /ui. ready backs /readyz — it should
// ping the underlying storage connection.
func NewRouter(
	metadataSvc metadataAPI, rowSvc rowAPI, schemaSvc schemaAPI, deriveSvc deriveAPI,
	geoSvc geoAPI, sourceSvc sourceAPI, exportSvc *service.ExportService,
	apiCfg config.APIConfig, uiAssets fs.FS, ready func(ctx context.Context) error,
) *Router {
	return &Router{
		tables:   newTablesHandler(metadataSvc, schemaSvc, deriveSvc, rowSvc, apiCfg),
		rows:     newRowsHandler(metadataSvc, rowSvc, apiCfg),
		geo:      newGeoHandler(metadataSvc, schemaSvc, geoSvc),
		sources:  newSourcesHandler(sourceSvc),
		export:   newExportHandler(metadataSvc, exportSvc),
		ops:      newOpsHandler(ready),
		apiCfg:   apiCfg,
		uiAssets: uiAssets,
	}
}

// Setup builds the chi router with every route from docs/spec.md section 6.1.
func (rt *Router) Setup() http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(slogLogger)
	r.Use(middleware.Recoverer)
	r.Use(maxBodyBytes(rt.apiCfg.MaxBodyBytes))

	r.Get("/healthz", rt.ops.healthz)
	r.Get("/readyz", rt.ops.readyz)

	r.Get("/swagger/*", httpSwagger.WrapHandler)

	r.Get("/", func(w http.ResponseWriter, req *http.Request) {
		http.Redirect(w, req, "/ui/", http.StatusFound)
	})
	r.Handle("/ui", http.RedirectHandler("/ui/", http.StatusFound))
	r.Handle("/ui/*", mountUI(rt.uiAssets))

	r.Route("/api/v1", func(v1 chi.Router) {
		// middleware.Timeout is scoped to this Group rather than applied at
		// the very top of Setup, so it never wraps exportTable below — that
		// route is registered outside the Group precisely to escape it
		// (NEW-BUG-7). Everything else keeps the same bounded-request budget
		// as before.
		v1.Group(func(v1 chi.Router) {
			if rt.apiCfg.RequestTimeoutSec > 0 {
				v1.Use(middleware.Timeout(time.Duration(rt.apiCfg.RequestTimeoutSec) * time.Second))
			}

			v1.Route("/sources", func(sub chi.Router) {
				sub.Get("/", rt.sources.listSources)
				sub.Post("/refresh", rt.sources.refreshSources)
				// Overrides the global body cap: an upload is a whole source file
				// (up to sources.maxSizeBytes), not a JSON payload. The extra 64KiB
				// covers multipart framing (boundary, headers) around the file part
				// itself, so a file right at the limit isn't cut off by framing
				// overhead before SourceService.Upload's own oversize check runs.
				sub.With(maxBodyBytes(rt.sources.sourceService.MaxUploadBytes()+64*1024)).Post("/upload", rt.sources.uploadSource)
				sub.Post("/{name}/load", rt.sources.loadSource)
				sub.Get("/{name}/metadata", rt.sources.getSourceMetadata)
				sub.Delete("/{name}", rt.sources.deleteSource)
			})

			v1.Route("/tables", func(sub chi.Router) {
				sub.Get("/", rt.tables.listTables)

				sub.Route("/{tableId}", func(sub chi.Router) {
					sub.Get("/", rt.tables.getTableMetadata)
					sub.Patch("/", rt.tables.renameTable)
					sub.Delete("/", rt.tables.deleteTable)

					sub.Get("/schema", rt.tables.getSchema)
					sub.Post("/columns", rt.tables.addColumn)
					sub.Post("/derive", rt.tables.deriveTable)
					sub.Post("/filter", rt.tables.filterRows)
					sub.Get("/columns/{col}/distinct", rt.tables.getDistinctValues)

					sub.Route("/rows", func(sub chi.Router) {
						sub.Get("/", rt.rows.listRows)
						sub.Post("/", rt.rows.addRows)
						sub.Route("/{pkey}", func(sub chi.Router) {
							sub.Get("/", rt.rows.getRow)
							sub.Patch("/", rt.rows.updateRow)
							sub.Delete("/", rt.rows.deleteRow)
						})
					})

					sub.Get("/tiles/{z}/{x}/{y}", rt.geo.getTile)
					sub.Get("/geometry-types", rt.geo.getGeometryTypes)
					sub.Post("/bbox", rt.geo.filterByBbox)
				})
			})
		})

		// exportTable streams a table in bounded batches (PERF-2) and can
		// legitimately run past any normal request's time budget on a large
		// table — middleware.Timeout would otherwise cancel it mid-stream,
		// truncating an already-200'd response (NEW-BUG-7).
		v1.Get("/tables/{tableId}/export", rt.export.exportTable)
	})

	return r
}
