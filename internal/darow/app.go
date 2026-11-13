// Package darow is the darow module hosted by the platform: tables loaded from geographic files into
// its DuckDB file, browsed, edited, filtered, drawn as vector tiles, exported and served as an OGC API
// - Features service; its saved filters live in its platform database.
package darow

import (
	"context"
	"path/filepath"

	"github.com/go-chi/chi/v5"

	"github.com/MyAppLibs/go-pkg/backup"
	"github.com/MyAppLibs/go-pkg/database"
	"github.com/MyAppLibs/go-pkg/events"
	"github.com/MyAppLibs/go-pkg/starter"

	"github.com/amauryval/darow/internal/darow/engine"
	"github.com/amauryval/darow/internal/darow/filters"
	"github.com/amauryval/darow/internal/darow/rest"
	"github.com/amauryval/darow/internal/darow/settings"
	"github.com/amauryval/darow/internal/darow/sources"
)

// Name is the module's name: its route prefix, its database and the base name of its DuckDB file.
const Name = "darow"

// App is the darow module.
type App struct {
	exportsDir string
	engine     *engine.Engine
	sources    *sources.Files
	filters    *filters.Store
	settings   *settings.Store
	server     *rest.Server
	events     events.Notifier
}

// Limits are the module's bounds: the largest source, DuckDB's memory in MiB (its own when 0) and
// the tiles rendered at once.
type Limits struct {
	SourceBytes int64
	MemoryMB    int
	Tiles       int
}

// New returns the module over workDir within limits, opened later by the starter. DuckDB reaches
// the sources and the exports' directory only.
func New(workDir string, limits Limits) *App {
	sourcesDir, exportsDir := filepath.Join(workDir, "sources"), filepath.Join(workDir, "exports")

	return &App{
		exportsDir: exportsDir,
		engine: engine.New(filepath.Join(workDir, Name+".duckdb"), engine.Limits{
			Dirs: []string{sourcesDir, exportsDir}, MemoryMB: limits.MemoryMB, Tiles: limits.Tiles,
		}),
		sources: sources.New(sourcesDir, limits.SourceBytes),
	}
}

// Name returns the module's name.
func (a *App) Name() string { return Name }

// Schemas returns the module's one schema: the saved filters, the settings, the filters' SQL.
func (a *App) Schemas() []database.Schema {
	return []database.Schema{{Name: Name, Migrations: []database.Migration{
		{Version: 1, Statement: filters.Schema},
		{Version: 2, Statement: settings.Schema},
		{Version: 3, Statement: filters.WhereSchema},
	}}}
}

// Attachments carries the DuckDB file in every archive of the module's database.
func (a *App) Attachments() []backup.Attachment {
	return []backup.Attachment{{Entry: filepath.Base(a.engine.Path()), Snapshot: a.engine.Snapshot, Restore: a.engine.Restore}}
}

// Open opens the DuckDB file and builds the filters and the routes.
func (a *App) Open(_ context.Context, host starter.Host) error {
	if err := a.engine.Open(); err != nil {
		return err
	}
	a.events = host.Events
	a.filters = filters.New(host.DB, host.Clock)
	a.filters.NotifyTo(host.Events, filters.Resource)
	a.settings = settings.New(host.DB)
	a.server = rest.NewServer(rest.Deps{
		Engine: a.engine, Sources: a.sources, Filters: a.filters, Settings: a.settings, Events: host.Events, TmpDir: a.exportsDir,
	})

	return nil
}

// RegisterRoutes mounts the module's routes under /api/v1/darow.
func (a *App) RegisterRoutes(r chi.Router) {
	a.server.Register(r)
}

// RegisterGuestRoutes mounts the writes of the data a guest may make: rows, columns, filters.
func (a *App) RegisterGuestRoutes(r chi.Router) {
	a.server.RegisterGuest(r)
}

// Erasures are the tables, with their filters and settings, and the filters alone.
func (a *App) Erasures() []starter.Erasure {
	return []starter.Erasure{
		{Key: "tables", Count: a.engine.Count, Erase: a.eraseTables},
		{Key: "filters", Count: func(ctx context.Context) (int, error) { return a.filters.DB().Count(ctx, "filters") }, Erase: a.eraseFilters},
	}
}

// BeforeRestore lets every restore and erasure through.
func (a *App) BeforeRestore(context.Context) error { return nil }

// AfterRestore tells the clients the tables and filters may have changed.
func (a *App) AfterRestore(ctx context.Context, _ bool) {
	a.events.Notify(ctx, rest.TablesResource, events.ActionChanged, "")
	a.events.Notify(ctx, filters.Resource, events.ActionChanged, "")
	a.events.Notify(ctx, settings.Resource, events.ActionChanged, "")
}

// Close closes the DuckDB file.
func (a *App) Close() error {
	return a.engine.Close()
}

func (a *App) eraseTables(ctx context.Context) error {
	if err := a.engine.DropAll(ctx); err != nil {
		return err
	}
	if err := a.settings.DropTables(ctx); err != nil {
		return err
	}

	return a.eraseFilters(ctx)
}

func (a *App) eraseFilters(ctx context.Context) error {
	return a.filters.DropAll(ctx)
}
