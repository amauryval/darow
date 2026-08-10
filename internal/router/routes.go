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
	"darow/pkg/auth"
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
	auth          *auth.Authenticator
	uiAssets      fs.FS
}

// NewRouter builds a Router serving every service in svcs — every row/list
// limit and geometry bound is read live off svcs.Settings.Current() by the
// handlers, so an update through PATCH /settings takes effect on the next
// request; the body cap and the request timeout are fixed at Setup instead,
// from settingsCfg (TODO-17 — a memory/availability guard, restart-only).
// uiAssets is the admin UI's built static files, served under /ui. ready
// backs /readyz — it should ping the underlying storage connection.
// authenticator guards every state-changing route; reads stay public.
func NewRouter(
	svcs *core.Services, uiAssets fs.FS, ready func(ctx context.Context) error,
	settingsCfg config.SettingsConfig, dbMaxSizeBytes int64, authenticator *auth.Authenticator,
) *Router {
	return &Router{
		tables:        handlers.NewTablesHandler(svcs.Metadata, svcs.Schema, svcs.Derive, svcs.Row, svcs.Column, svcs.Settings, svcs.TableSettings),
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
		auth:          authenticator,
		uiAssets:      uiAssets,
	}
}

// Setup builds the chi router with every route from spec/back/spec.md section 6.1.
func (rt *Router) Setup() http.Handler {
	r := chi.NewRouter()

	r.NotFound(notFoundJSON)
	r.MethodNotAllowed(methodNotAllowedJSON)

	r.Use(middleware.RequestID)
	r.Use(rt.auth.Authenticate)
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
			upload.Use(rt.auth.Authenticated, rt.auth.CanWrite)
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
				sub.Get("/{name}/metadata", rt.sources.GetSourceMetadata)

				sub.Group(func(w chi.Router) {
					w.Use(rt.auth.Authenticated, rt.auth.CanWrite)
					w.Post("/{name}/load", rt.sources.LoadSource)
					w.Delete("/{name}", rt.sources.DeleteSource)
				})
			})

			v1.Group(func(w chi.Router) {
				w.Use(rt.auth.Authenticated, rt.auth.CanWrite)
				w.Post("/derive", rt.tables.DeriveTable)
			})

			v1.Route("/tables", func(sub chi.Router) {
				sub.Get("/", rt.tables.ListTables)

				sub.Route("/{tableId}", func(sub chi.Router) {
					sub.Get("/", rt.tables.GetTableMetadata)
					sub.Get("/schema", rt.tables.GetSchema)
					sub.Post("/filter", rt.tables.FilterRows)

					sub.Group(func(w chi.Router) {
						w.Use(rt.auth.Authenticated, rt.auth.CanWrite)
						w.Patch("/", rt.tables.RenameTable)
						w.Delete("/", rt.tables.DeleteTable)
						w.Post("/derive-from-table", rt.tables.DeriveFromTable)
						w.Post("/aggregate-from-table", rt.tables.AggregateFromTable)
					})

					sub.Route("/settings", func(sub chi.Router) {
						sub.Get("/", rt.tableSettings.GetTableSettings)

						sub.Group(func(w chi.Router) {
							w.Use(rt.auth.Authenticated, rt.auth.CanWrite)
							w.Patch("/", rt.tableSettings.UpdateTableSettings)
							w.Delete("/", rt.tableSettings.DeleteTableSettings)
						})
					})

					sub.Route("/filters", func(sub chi.Router) {
						sub.Get("/", rt.filters.ListFilters)

						sub.Group(func(w chi.Router) {
							w.Use(rt.auth.Authenticated, rt.auth.CanWrite)
							w.Post("/", rt.filters.CreateFilter)
							w.Patch("/{filterId}", rt.filters.UpdateFilter)
							w.Delete("/{filterId}", rt.filters.DeleteFilter)
						})
					})
					sub.Route("/columns", func(sub chi.Router) {
						sub.Get("/{col}/distinct", rt.tables.GetDistinctValues)
						sub.Post("/{col}/transform/preview", rt.tables.PreviewColumnTransform)

						sub.Group(func(w chi.Router) {
							w.Use(rt.auth.Authenticated, rt.auth.CanWrite)
							w.Post("/", rt.tables.AddColumn)
							w.Post("/{col}/transform", rt.tables.TransformColumn)
						})
					})

					sub.Route("/rows", func(sub chi.Router) {
						sub.Get("/", rt.rows.ListRows)
						sub.Post("/batch", rt.rows.GetRowsByPkey)

						sub.Group(func(w chi.Router) {
							w.Use(rt.auth.Authenticated, rt.auth.CanWrite)
							w.Post("/", rt.rows.AddRows)
							w.Patch("/", rt.rows.UpdateRows)
						})

						sub.Route("/{pkey}", func(sub chi.Router) {
							sub.Get("/", rt.rows.GetRow)
							sub.Get("/geometry", rt.rows.GetRowGeometry)

							sub.Group(func(w chi.Router) {
								w.Use(rt.auth.Authenticated, rt.auth.CanWrite)
								w.Patch("/", rt.rows.UpdateRow)
								w.Delete("/", rt.rows.DeleteRow)
							})
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

			// The endpoint is the auth package's own (path excepted, which
			// is ours): it answers with the caller that package resolved.
			v1.Get("/session", rt.auth.ServeSession)

			v1.Route("/settings", func(sub chi.Router) {
				sub.Get("/", rt.settings.GetSettings)
				sub.Get("/tables", rt.tableSettings.ListOverriddenTables)

				sub.Group(func(w chi.Router) {
					w.Use(rt.auth.Authenticated, rt.auth.CanWrite)
					w.Patch("/", rt.settings.UpdateSettings)
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
		// table — requestTimeout would otherwise cancel it mid-stream,
		// truncating an already-200'd response.
		v1.Get("/tables/{tableId}/export", rt.export.ExportTable)
	})

	return r
}
