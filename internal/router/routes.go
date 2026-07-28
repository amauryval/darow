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

// Router wires the HTTP layer to the service layer, depending on each
// service through the narrow port it actually calls (metadataAPI, rowAPI,
// schemaAPI, deriveAPI, geoAPI, sourceAPI — see ports.go) rather than the
// concrete service types.
type Router struct {
	metadataService metadataAPI
	rowService      rowAPI
	schemaService   schemaAPI
	deriveService   deriveAPI
	geoService      geoAPI
	sourceService   sourceAPI
	exportService   *service.ExportService
	apiCfg          config.APIConfig
	uiAssets        fs.FS
	ready           func(ctx context.Context) error
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
		metadataService: metadataSvc, rowService: rowSvc, schemaService: schemaSvc, deriveService: deriveSvc,
		geoService: geoSvc, sourceService: sourceSvc, exportService: exportSvc,
		apiCfg: apiCfg, uiAssets: uiAssets, ready: ready,
	}
}

// Setup builds the chi router with every route from docs/spec.md section 6.1.
func (rt *Router) Setup() http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(slogLogger)
	r.Use(middleware.Recoverer)
	if rt.apiCfg.RequestTimeoutSec > 0 {
		r.Use(middleware.Timeout(time.Duration(rt.apiCfg.RequestTimeoutSec) * time.Second))
	}
	r.Use(maxBodyBytes(rt.apiCfg.MaxBodyBytes))

	r.Get("/healthz", rt.healthz)
	r.Get("/readyz", rt.readyz)

	r.Get("/swagger/*", httpSwagger.WrapHandler)

	r.Get("/", func(w http.ResponseWriter, req *http.Request) {
		http.Redirect(w, req, "/ui/", http.StatusFound)
	})
	r.Handle("/ui", http.RedirectHandler("/ui/", http.StatusFound))
	r.Handle("/ui/*", mountUI(rt.uiAssets))

	r.Route("/api/v1", func(v1 chi.Router) {
		v1.Route("/sources", func(sub chi.Router) {
			sub.Get("/", rt.listSources)
			sub.Post("/refresh", rt.refreshSources)
			// Overrides the global body cap: an upload is a whole source file
			// (up to sources.maxSizeBytes), not a JSON payload. The extra 64KiB
			// covers multipart framing (boundary, headers) around the file part
			// itself, so a file right at the limit isn't cut off by framing
			// overhead before SourceService.Upload's own oversize check runs.
			sub.With(maxBodyBytes(rt.sourceService.MaxUploadBytes()+64*1024)).Post("/upload", rt.uploadSource)
			sub.Post("/{name}/load", rt.loadSource)
			sub.Get("/{name}/metadata", rt.getSourceMetadata)
			sub.Delete("/{name}", rt.deleteSource)
		})

		v1.Route("/tables", func(sub chi.Router) {
			sub.Get("/", rt.listTables)

			sub.Route("/{tableId}", func(sub chi.Router) {
				sub.Use(rt.tableMiddleware)

				sub.Get("/", rt.getTableMetadata)
				sub.Patch("/", rt.renameTable)
				sub.Delete("/", rt.deleteTable)

				sub.Get("/schema", rt.getSchema)
				sub.Post("/columns", rt.addColumn)
				sub.Post("/derive", rt.deriveTable)
				sub.Get("/export", rt.exportTable)
				sub.Post("/filter", rt.filterRows)
				sub.Get("/columns/{col}/distinct", rt.getDistinctValues)

				sub.Route("/rows", func(sub chi.Router) {
					sub.Get("/", rt.listRows)
					sub.Post("/", rt.addRows)
					sub.Route("/{pkey}", func(sub chi.Router) {
						sub.Get("/", rt.getRow)
						sub.Patch("/", rt.updateRow)
						sub.Delete("/", rt.deleteRow)
					})
				})

				sub.Get("/tiles/{z}/{x}/{y}", rt.getTile)
				sub.Get("/geometry-types", rt.getGeometryTypes)
				sub.Post("/bbox", rt.filterByBbox)
			})
		})
	})

	// Backward-compatible redirects from the unversioned root (HTTP-2 —
	// versioned API) while the front finishes migrating to /api/v1.
	// StatusPermanentRedirect (308), unlike 301/302, keeps the original
	// method and body, so a POST/PATCH/DELETE redirects correctly too.
	r.Handle("/sources", http.HandlerFunc(redirectToAPIv1))
	r.Handle("/sources/*", http.HandlerFunc(redirectToAPIv1))
	r.Handle("/tables", http.HandlerFunc(redirectToAPIv1))
	r.Handle("/tables/*", http.HandlerFunc(redirectToAPIv1))

	return r
}

func redirectToAPIv1(w http.ResponseWriter, req *http.Request) {
	target := "/api/v1" + req.URL.Path
	if req.URL.RawQuery != "" {
		target += "?" + req.URL.RawQuery
	}
	http.Redirect(w, req, target, http.StatusPermanentRedirect)
}
