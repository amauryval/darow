package router

import (
	"io/fs"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	httpSwagger "github.com/swaggo/http-swagger"

	_ "gogeoserver/docs"
	"gogeoserver/internal/config"
	"gogeoserver/internal/service"
)

// Router wires the HTTP layer to the service layer.
type Router struct {
	tableService  *service.TableService
	geoService    *service.GeoService
	sourceService *service.SourceService
	apiCfg        config.APIConfig
	uiAssets      fs.FS
}

// NewRouter builds a Router serving tableSvc, geoSvc, and sourceSvc,
// honoring apiCfg's row pagination limits (règle 18). uiAssets is the
// admin UI's built static files, served under /ui.
func NewRouter(tableSvc *service.TableService, geoSvc *service.GeoService, sourceSvc *service.SourceService, apiCfg config.APIConfig, uiAssets fs.FS) *Router {
	return &Router{tableService: tableSvc, geoService: geoSvc, sourceService: sourceSvc, apiCfg: apiCfg, uiAssets: uiAssets}
}

// Setup builds the chi router with every route from docs/spec.md section 6.1.
func (rt *Router) Setup() http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Get("/swagger/*", httpSwagger.WrapHandler)

	r.Get("/", func(w http.ResponseWriter, req *http.Request) {
		http.Redirect(w, req, "/ui/", http.StatusFound)
	})
	r.Handle("/ui", http.RedirectHandler("/ui/", http.StatusFound))
	r.Handle("/ui/*", mountUI(rt.uiAssets))

	r.Route("/sources", func(sub chi.Router) {
		sub.Get("/", rt.listSources)
		sub.Post("/refresh", rt.refreshSources)
		sub.Post("/upload", rt.uploadSource)
		sub.Post("/{name}/load", rt.loadSource)
	})

	r.Route("/tables", func(sub chi.Router) {
		sub.Get("/", rt.listTables)

		sub.Route("/{tableId}", func(sub chi.Router) {
			sub.Use(rt.tableMiddleware)

			sub.Get("/", rt.getTableMetadata)
			sub.Delete("/", rt.deleteTable)

			sub.Get("/schema", rt.getSchema)
			sub.Post("/columns", rt.addColumn)
			sub.Post("/derive", rt.deriveTable)
			sub.Get("/export", rt.exportTable)

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
			sub.Post("/bbox", rt.filterByBbox)
		})
	})

	return r
}
