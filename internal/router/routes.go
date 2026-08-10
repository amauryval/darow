package router

import (
	"context"
	"io/fs"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	httpSwagger "github.com/swaggo/http-swagger"

	_ "darow/docs"
	"darow/internal/config"
	"darow/internal/core"
	"darow/internal/router/handlers"
)

// Router wires the HTTP layer to the service layer. Each family of
// endpoints (tables, rows, geo, sources, export, ops, settings) is served
// by its own handler struct, holding only the service ports it actually
// needs — never the concrete service types.
type Router struct {
	tables        *handlers.TablesHandler
	rows          *handlers.RowsHandler
	geo           *handlers.GeoHandler
	features      *handlers.FeaturesHandler
	sources       *handlers.SourcesHandler
	filters       *handlers.FiltersHandler
	export        *handlers.ExportHandler
	ops           *handlers.OpsHandler
	settings      *handlers.SettingsHandler
	tableSettings *handlers.TableSettingsHandler
	settingsCfg   config.SettingsConfig
	uiAssets      fs.FS
}

// NewRouter builds a Router serving every service in svcs — every row/list
// limit and geometry bound is read live off svcs.Settings.Current() by the
// handlers, so an update through PATCH /settings takes effect on the next
// request; the body cap and the request timeout are fixed at Setup instead,
// from settingsCfg (TODO-17 — a memory/availability guard, restart-only).
// uiAssets is the admin UI's built static files, served under /ui. ready
// backs /readyz — it should ping the underlying storage connection.
func NewRouter(
	svcs *core.Services, uiAssets fs.FS, ready func(ctx context.Context) error,
	settingsCfg config.SettingsConfig, dbMaxSizeBytes int64,
) *Router {
	return &Router{
		tables:        handlers.NewTablesHandler(svcs.Metadata, svcs.Schema, svcs.Derive, svcs.Row, svcs.Settings, svcs.TableSettings),
		rows:          handlers.NewRowsHandler(svcs.Metadata, svcs.Schema, svcs.Row, svcs.Geo, svcs.Settings, svcs.TableSettings),
		geo:           handlers.NewGeoHandler(svcs.Metadata, svcs.Schema, svcs.Geo, svcs.Settings, svcs.TableSettings),
		features:      handlers.NewFeaturesHandler(svcs.Metadata, svcs.Schema, svcs.Geo, svcs.Settings, svcs.TableSettings),
		sources:       handlers.NewSourcesHandler(svcs.Source, svcs.Settings),
		filters:       handlers.NewFiltersHandler(svcs.Metadata, svcs.Filter),
		export:        handlers.NewExportHandler(svcs.Metadata, svcs.Export),
		ops:           handlers.NewOpsHandler(ready, svcs.Diagnostics, dbMaxSizeBytes),
		settings:      handlers.NewSettingsHandler(svcs.Settings),
		tableSettings: handlers.NewTableSettingsHandler(svcs.Metadata, svcs.TableSettings),
		settingsCfg:   settingsCfg,
		uiAssets:      uiAssets,
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

	r.Get("/healthz", rt.ops.Healthz)
	r.Get("/readyz", rt.ops.Readyz)
	r.Get("/diagnostics", rt.ops.Diagnostics)

	r.Get("/swagger/*", httpSwagger.WrapHandler)

	r.Get("/", func(w http.ResponseWriter, req *http.Request) {
		http.Redirect(w, req, "/ui/", http.StatusFound)
	})
	r.Handle("/ui", http.RedirectHandler("/ui/", http.StatusFound))
	r.Handle("/ui/*", mountUI(rt.uiAssets))

	r.Route("/api/v1", func(v1 chi.Router) {
		// An upload is a whole source file, not a JSON payload, and is
		// registered outside the Group below rather than inside it: stacking
		// body caps applies the *smaller* of the two, never the more
		// specific, so a per-route cap nested under the global one is no
		// override at all. The same registration keeps requestTimeout off it
		// — a multi-gigabyte body takes longer to read than any normal
		// request's budget. The extra 64KiB covers multipart framing
		// (boundary, headers) around the file part, so a file right at the
		// limit isn't cut off before SourceService.Upload's own oversize
		// check runs.
		v1.Group(func(upload chi.Router) {
			upload.Use(maxBodyBytes(func() int64 { return rt.sources.MaxUploadBytes() + 64*1024 }))
			upload.Post("/sources/upload", rt.sources.UploadSource)
		})

		// requestTimeout and the global body cap are scoped to this Group
		// rather than applied at the very top of Setup, so they never wrap
		// exportTable or the upload above — both are registered outside it
		// precisely to escape them.
		v1.Group(func(v1 chi.Router) {
			v1.Use(requestTimeout(func() int { return rt.settingsCfg.RequestTimeoutSec }))
			v1.Use(maxBodyBytes(func() int64 { return rt.settingsCfg.MaxBodyBytes }))

			v1.Route("/sources", func(sub chi.Router) {
				sub.Get("/", rt.sources.ListSources)
				sub.Post("/refresh", rt.sources.RefreshSources)
				sub.Post("/{name}/load", rt.sources.LoadSource)
				sub.Get("/{name}/metadata", rt.sources.GetSourceMetadata)
				sub.Delete("/{name}", rt.sources.DeleteSource)
			})

			v1.Post("/derive", rt.tables.DeriveTable)

			v1.Route("/tables", func(sub chi.Router) {
				sub.Get("/", rt.tables.ListTables)

				sub.Route("/{tableId}", func(sub chi.Router) {
					sub.Get("/", rt.tables.GetTableMetadata)
					sub.Patch("/", rt.tables.RenameTable)
					sub.Delete("/", rt.tables.DeleteTable)

					sub.Get("/schema", rt.tables.GetSchema)
					sub.Post("/columns", rt.tables.AddColumn)
					sub.Post("/derive-from-table", rt.tables.DeriveFromTable)
					sub.Post("/filter", rt.tables.FilterRows)

					sub.Route("/settings", func(sub chi.Router) {
						sub.Get("/", rt.tableSettings.GetTableSettings)
						sub.Patch("/", rt.tableSettings.UpdateTableSettings)
						sub.Delete("/", rt.tableSettings.DeleteTableSettings)
					})

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
						sub.Patch("/", rt.rows.UpdateRows)
						sub.Post("/batch", rt.rows.GetRowsByPkey)
						sub.Route("/{pkey}", func(sub chi.Router) {
							sub.Get("/", rt.rows.GetRow)
							sub.Patch("/", rt.rows.UpdateRow)
							sub.Delete("/", rt.rows.DeleteRow)
							sub.Get("/geometry", rt.rows.GetRowGeometry)
						})
					})

					sub.Get("/tiles/{z}/{x}/{y}", rt.geo.GetTile)
					sub.Get("/tiles/{z}/{x}/{y}/features", rt.geo.GetTileFeatures)
					sub.Get("/geometry-types", rt.geo.GetGeometryTypes)
					sub.Post("/bbox", rt.geo.FilterByBbox)
					sub.Post("/neighbours", rt.geo.FindNeighbours)
					sub.Get("/filtered-extent", rt.geo.GetFilteredExtent)
				})
			})

			v1.Route("/settings", func(sub chi.Router) {
				sub.Get("/", rt.settings.GetSettings)
				sub.Patch("/", rt.settings.UpdateSettings)
				sub.Get("/tables", rt.tableSettings.ListOverriddenTables)
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
		// table — requestTimeout would otherwise cancel it mid-stream,
		// truncating an already-200'd response.
		v1.Get("/tables/{tableId}/export", rt.export.ExportTable)
	})

	return r
}
