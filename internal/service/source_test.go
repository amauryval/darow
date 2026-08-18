package service

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"darow/internal/config"
	"darow/internal/storage/mocks"
)

func writeFixture(t *testing.T, dir, name string, size int) {
	path := filepath.Join(dir, "sources", name)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, make([]byte, size), 0o644))
}

func setupSourceService(t *testing.T, dir string, maxSize int64, extensions ...string) (*SourceService, *mocks.Adapter) {
	serverConfig := config.ServerConfig{WorkDir: dir}
	require.NoError(t, os.MkdirAll(serverConfig.SourcesPath(), 0o755))
	tableSvc, m := setupImportService(t)
	svc := NewSourceService(
		serverConfig,
		config.SourcesConfig{MaxSizeBytes: maxSize, AllowedExtensions: extensions},
		tableSvc)
	return svc, m
}

func sourcePathOf(dir, name string) string {
	return filepath.Join(dir, "sources", name)
}

func Test_SourceService_Refresh(t *testing.T) {
	dir := t.TempDir()
	writeFixture(t, dir, "ok.geojson", 100)
	writeFixture(t, dir, "too_big.geojson", 200)
	writeFixture(t, dir, "ignored.txt", 100)

	svc, _ := setupSourceService(t, dir, 150, ".geojson")

	sources, err := svc.Refresh(context.Background())

	require.NoError(t, err)
	require.Len(t, sources, 1)
	assert.Equal(t, "ok.geojson", sources[0].Name)
	assert.Empty(t, sources[0].Layers)
}

func Test_SourceService_Refresh_PopulatesGpkgLayers(t *testing.T) {
	dir := t.TempDir()
	writeFixture(t, dir, "regions.gpkg", 100)

	svc, m := setupSourceService(t, dir, 1000, ".gpkg")
	m.On("ListLayers", mock.Anything, sourcePathOf(dir, "regions.gpkg")).Return([]string{"communes", "departements"}, nil).Once()

	sources, err := svc.Refresh(context.Background())

	require.NoError(t, err)
	require.Len(t, sources, 1)
	assert.Equal(t, []string{"communes", "departements"}, sources[0].Layers)
}

func Test_SourceService_Upload(t *testing.T) {
	cases := []struct {
		name         string
		filename     string
		content      string
		preexisting  bool
		maxSize      int64
		wantErr      bool
		wantConflict bool
		wantSaved    string
	}{
		{
			name:      "writes file and sanitizes traversal",
			filename:  "../inputData/tutu.geojson",
			content:   `{"type":"FeatureCollection","features":[]}`,
			maxSize:   1000,
			wantSaved: "tutu.geojson",
		},
		{
			name:     "rejects disallowed extension",
			filename: "lala.exe",
			content:  "x",
			maxSize:  1000,
			wantErr:  true,
		},
		{
			name:         "rejects already existing file",
			filename:     "buildings.geojson",
			content:      "x",
			preexisting:  true,
			maxSize:      1000,
			wantErr:      true,
			wantConflict: true,
		},
		{
			name:     "rejects content exceeding maxSizeBytes",
			filename: "big.geojson",
			content:  "0123456789",
			maxSize:  5,
			wantErr:  true,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			if c.preexisting {
				writeFixture(t, dir, "buildings.geojson", 10)
			}
			svc, _ := setupSourceService(t, dir, c.maxSize, ".geojson")

			saved, err := svc.Upload(c.filename, strings.NewReader(c.content))

			if c.wantErr {
				assert.Error(t, err)
				if c.wantConflict {
					var conflictErr *ConflictError
					assert.ErrorAs(t, err, &conflictErr, "an already-existing file is the caller's mistake, not an internal failure")
				}
				return
			}
			require.NoError(t, err)
			assert.Equal(t, c.wantSaved, saved)

			data, err := os.ReadFile(sourcePathOf(dir, saved))
			require.NoError(t, err)
			assert.Equal(t, c.content, string(data))
		})
	}
}

func Test_SourceService_Load(t *testing.T) {
	cases := []struct {
		name           string
		file           string
		extension      string
		layers         []string
		requestedName  string
		requestedName2 string
		layer          string
		srid           int
		wantTableName  string
		wantErr        bool
	}{
		{
			name:          "rejects a name absent from the cache",
			file:          "buildings.geojson",
			extension:     ".geojson",
			requestedName: "../../etc/passwd",
			wantErr:       true,
		},
		{
			name:          "imports a single-layer source",
			file:          "buildings.geojson",
			extension:     ".geojson",
			requestedName: "buildings.geojson",
			wantTableName: "buildings.geojson",
		},
		{
			name:          "forwards an explicit srid",
			file:          "lambert.geojson",
			extension:     ".geojson",
			requestedName: "lambert.geojson",
			srid:          2154,
			wantTableName: "lambert.geojson",
		},
		{
			name:          "rejects a layer on a single-layer format",
			file:          "buildings.geojson",
			extension:     ".geojson",
			requestedName: "buildings.geojson",
			layer:         "some_layer",
			wantErr:       true,
		},
		{
			name:          "imports one layer of a multi-layer source",
			file:          "regions.gpkg",
			extension:     ".gpkg",
			layers:        []string{"communes", "departements"},
			requestedName: "regions.gpkg",
			layer:         "communes",
			wantTableName: "regions.gpkg::communes",
		},
		{
			name:          "requires a layer on a multi-layer source",
			file:          "regions.gpkg",
			extension:     ".gpkg",
			layers:        []string{"communes"},
			requestedName: "regions.gpkg",
			wantErr:       true,
		},
		{
			name:          "rejects a layer absent from the cache",
			file:          "regions.gpkg",
			extension:     ".gpkg",
			layers:        []string{"communes"},
			requestedName: "regions.gpkg",
			layer:         "unknown",
			wantErr:       true,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			writeFixture(t, dir, c.file, 100)

			svc, m := setupSourceService(t, dir, 1000, c.extension)
			if c.layers != nil {
				m.On("ListLayers", mock.Anything, sourcePathOf(dir, c.file)).Return(c.layers, nil).Once()
			}
			_, err := svc.Refresh(context.Background())
			require.NoError(t, err)

			if !c.wantErr {
				m.On("CreateFromFile", mock.Anything, c.wantTableName, c.file, sourcePathOf(dir, c.file), c.layer, c.srid).Return("id-new", nil).Once()
			}

			tableId, err := svc.Load(context.Background(), c.requestedName, c.layer, "", c.srid)

			if c.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, "id-new", tableId)
		})
	}
}

// A loaded table is a copy of the file's data, independent of it: the same
// source may be loaded again, and each load is its own table.
func Test_SourceService_Load_SameSourceTwice(t *testing.T) {
	dir := t.TempDir()
	writeFixture(t, dir, "buildings.geojson", 100)

	svc, m := setupSourceService(t, dir, 1000, ".geojson")
	_, err := svc.Refresh(context.Background())
	require.NoError(t, err)

	m.On("CreateFromFile", mock.Anything, "buildings.geojson", "buildings.geojson", sourcePathOf(dir, "buildings.geojson"), "", 0).Return("id-1", nil).Once()
	m.On("CreateFromFile", mock.Anything, "buildings.geojson", "buildings.geojson", sourcePathOf(dir, "buildings.geojson"), "", 0).Return("id-2", nil).Once()

	first, err := svc.Load(context.Background(), "buildings.geojson", "", "", 0)
	require.NoError(t, err)
	second, err := svc.Load(context.Background(), "buildings.geojson", "", "", 0)
	require.NoError(t, err)

	assert.Equal(t, "id-1", first)
	assert.Equal(t, "id-2", second)
}

func Test_SourceService_List_Paginates(t *testing.T) {
	dir := t.TempDir()
	writeFixture(t, dir, "a.geojson", 10)
	writeFixture(t, dir, "b.geojson", 10)
	writeFixture(t, dir, "c.geojson", 10)

	svc, _ := setupSourceService(t, dir, 1000, ".geojson")
	_, err := svc.Refresh(context.Background())
	require.NoError(t, err)

	cases := []struct {
		name      string
		offset    int
		limit     int
		wantNames []string
	}{
		{name: "first page", offset: 0, limit: 2, wantNames: []string{"a.geojson", "b.geojson"}},
		{name: "second page", offset: 2, limit: 2, wantNames: []string{"c.geojson"}},
		{name: "offset beyond total", offset: 10, limit: 2, wantNames: []string{}},
		{name: "negative offset clamps to zero", offset: -5, limit: 1, wantNames: []string{"a.geojson"}},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			sources, total, err := svc.List(nil, c.offset, c.limit)
			require.NoError(t, err)

			assert.Equal(t, 3, total)
			names := make([]string, len(sources))
			for i, s := range sources {
				names[i] = s.Name
			}
			assert.Equal(t, c.wantNames, names)
		})
	}
}

// List must hand out an independent snapshot: mutating the returned slice
// (or a Source's Layers within it) must never reach back into the cache
// Load reads from.
func Test_SourceService_List_ReturnsIndependentCopy(t *testing.T) {
	dir := t.TempDir()
	writeFixture(t, dir, "regions.gpkg", 100)

	svc, m := setupSourceService(t, dir, 1000, ".gpkg")
	m.On("ListLayers", mock.Anything, sourcePathOf(dir, "regions.gpkg")).Return([]string{"communes"}, nil).Once()
	_, err := svc.Refresh(context.Background())
	require.NoError(t, err)

	sources, total, err := svc.List(nil, 0, 10)
	require.NoError(t, err)
	require.Len(t, sources, 1)
	require.Equal(t, 1, total)
	sources[0].Name = "tampered"
	sources[0].Layers[0] = "tampered"

	unchanged, _, err := svc.List(nil, 0, 10)
	require.NoError(t, err)
	assert.Equal(t, "regions.gpkg", unchanged[0].Name)
	assert.Equal(t, []string{"communes"}, unchanged[0].Layers)
}

func Test_SourceService_DetectCRS(t *testing.T) {
	dir := t.TempDir()
	writeFixture(t, dir, "lambert.geojson", 100)

	svc, m := setupSourceService(t, dir, 1000, ".geojson")
	_, err := svc.Refresh(context.Background())
	require.NoError(t, err)

	m.On("DetectCRS", mock.Anything, sourcePathOf(dir, "lambert.geojson"), "").Return("EPSG", 2154, nil).Once()

	authName, authCode, err := svc.DetectCRS(context.Background(), "lambert.geojson", "")

	require.NoError(t, err)
	assert.Equal(t, "EPSG", authName)
	assert.Equal(t, 2154, authCode)
}

// A non-Introspectable format (Parquet/GeoParquet) must never reach storage's
// DetectCRS — the ST_Read_Meta path every other format uses crashes on one.
// The mock has no "DetectCRS" expectation, so a call through would fail here.
func Test_SourceService_DetectCRS_SkipsNonIntrospectableFormat(t *testing.T) {
	dir := t.TempDir()
	writeFixture(t, dir, "widgets.parquet", 100)

	svc, _ := setupSourceService(t, dir, 1000, ".parquet")
	_, err := svc.Refresh(context.Background())
	require.NoError(t, err)

	authName, authCode, err := svc.DetectCRS(context.Background(), "widgets.parquet", "")

	require.NoError(t, err)
	assert.Empty(t, authName)
	assert.Zero(t, authCode)
}

func Test_SourceService_DetectCRS_WrapsStorageFailureInDisplayError(t *testing.T) {
	dir := t.TempDir()
	writeFixture(t, dir, "lambert.geojson", 100)

	svc, m := setupSourceService(t, dir, 1000, ".geojson")
	_, err := svc.Refresh(context.Background())
	require.NoError(t, err)

	cause := errors.New("duckdb: connection reset")
	m.On("DetectCRS", mock.Anything, sourcePathOf(dir, "lambert.geojson"), "").Return("", 0, cause).Once()

	_, _, err = svc.DetectCRS(context.Background(), "lambert.geojson", "")

	var displayErr *DisplayError
	require.ErrorAs(t, err, &displayErr)
	assert.Contains(t, displayErr.DisplayMessage(), "lambert.geojson")
}

func Test_SourceService_DetectCRS_RejectsUnknownSource(t *testing.T) {
	svc, _ := setupSourceService(t, t.TempDir(), 1000, ".geojson")

	_, _, err := svc.DetectCRS(context.Background(), "../../etc/passwd", "")

	assert.Error(t, err)
}

func Test_SourceService_SourceHasGeometry(t *testing.T) {
	cases := []struct {
		name        string
		file        string
		extension   string
		hasGeometry bool
		want        bool
	}{
		{name: "a geojson carrying a geometry", file: "points.geojson", extension: ".geojson", hasGeometry: true, want: true},
		{name: "a csv of plain columns", file: "plain.csv", extension: ".csv", want: false},
		{name: "a parquet is asked like any other format", file: "widgets.parquet", extension: ".parquet", want: false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			writeFixture(t, dir, c.file, 100)

			svc, m := setupSourceService(t, dir, 1000, c.extension)
			_, err := svc.Refresh(context.Background())
			require.NoError(t, err)

			m.On("SourceHasGeometry", mock.Anything, sourcePathOf(dir, c.file), "").Return(c.hasGeometry, nil).Once()

			got, err := svc.SourceHasGeometry(context.Background(), c.file, "")

			require.NoError(t, err)
			assert.Equal(t, c.want, got)
		})
	}
}

// Delete erases the file and forgets it, and stops there: the tables loaded
// from it hold their own copy of the data and must survive their source.
func Test_SourceService_Delete(t *testing.T) {
	dir := t.TempDir()
	writeFixture(t, dir, "buildings.geojson", 100)

	svc, _ := setupSourceService(t, dir, 1000, ".geojson")
	_, err := svc.Refresh(context.Background())
	require.NoError(t, err)

	err = svc.Delete("buildings.geojson")

	require.NoError(t, err)
	_, statErr := os.Stat(sourcePathOf(dir, "buildings.geojson"))
	assert.True(t, os.IsNotExist(statErr))
	remaining, total, err := svc.List(nil, 0, 10)
	require.NoError(t, err)
	assert.Empty(t, remaining)
	assert.Zero(t, total)
}

// A file removed from workDir behind the server's back still lets its cache
// entry be dropped: the outcome asked for is already the one on disk.
func Test_SourceService_Delete_FileAlreadyGone(t *testing.T) {
	dir := t.TempDir()
	writeFixture(t, dir, "buildings.geojson", 100)

	svc, _ := setupSourceService(t, dir, 1000, ".geojson")
	_, err := svc.Refresh(context.Background())
	require.NoError(t, err)
	require.NoError(t, os.Remove(sourcePathOf(dir, "buildings.geojson")))

	require.NoError(t, svc.Delete("buildings.geojson"))

	_, total, err := svc.List(nil, 0, 10)
	require.NoError(t, err)
	assert.Zero(t, total)
}

func Test_SourceService_Delete_RejectsUnknownSource(t *testing.T) {
	svc, _ := setupSourceService(t, t.TempDir(), 1000, ".geojson")

	err := svc.Delete("unknown.geojson")

	assert.Error(t, err)
}

// Delete mutates the cache slice a concurrent List reads — go test -race only
// catches it when both run genuinely concurrently, hence the List loop.
func Test_SourceService_Delete_ConcurrentWithList_NoRace(t *testing.T) {
	dir := t.TempDir()
	writeFixture(t, dir, "regions.gpkg", 100)

	svc, m := setupSourceService(t, dir, 1000, ".gpkg")
	m.On("ListLayers", mock.Anything, sourcePathOf(dir, "regions.gpkg")).Return([]string{"communes", "departements"}, nil).Once()
	_, err := svc.Refresh(context.Background())
	require.NoError(t, err)

	stop := make(chan struct{})
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		for {
			select {
			case <-stop:
				return
			default:
			}
			if _, _, err := svc.List(nil, 0, 10); err != nil {
				return
			}
		}
	}()

	require.NoError(t, svc.Delete("regions.gpkg"))
	close(stop)
	<-finished
}

func Test_SourceService_Load_TakesAnExplicitTableName(t *testing.T) {
	dir := t.TempDir()
	writeFixture(t, dir, "buildings.geojson", 100)

	svc, m := setupSourceService(t, dir, 1000, ".geojson")
	_, err := svc.Refresh(context.Background())
	require.NoError(t, err)

	m.On("CreateFromFile", mock.Anything, "bâtiments 2024", "buildings.geojson", sourcePathOf(dir, "buildings.geojson"), "", 0).Return("id-1", nil).Once()

	tableId, err := svc.Load(context.Background(), "buildings.geojson", "", "bâtiments 2024", 0)

	require.NoError(t, err)
	assert.Equal(t, "id-1", tableId)
}

func Test_SourceService_Reload(t *testing.T) {
	cases := []struct {
		name       string
		file       string
		extension  string
		layers     []string
		layer      string
		srid       int
		wantSource string
		wantErr    bool
	}{
		{name: "reloads from a single-layer source", file: "buildings.geojson", extension: ".geojson", wantSource: "buildings.geojson"},
		{name: "reloads from one layer", file: "regions.gpkg", extension: ".gpkg", layers: []string{"communes"}, layer: "communes", wantSource: "regions.gpkg"},
		{name: "forwards the srid override", file: "lambert.geojson", extension: ".geojson", srid: 2154, wantSource: "lambert.geojson"},
		{name: "rejects a layer absent from the cache", file: "regions.gpkg", extension: ".gpkg", layers: []string{"communes"}, layer: "unknown", wantErr: true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			writeFixture(t, dir, c.file, 100)

			svc, m := setupSourceService(t, dir, 1000, c.extension)
			if c.layers != nil {
				m.On("ListLayers", mock.Anything, sourcePathOf(dir, c.file)).Return(c.layers, nil).Once()
			}
			_, err := svc.Refresh(context.Background())
			require.NoError(t, err)

			if !c.wantErr {
				m.On("ReloadFromFile", mock.Anything, "t1", c.wantSource, sourcePathOf(dir, c.file), c.layer, c.srid).Return(nil).Once()
			}

			err = svc.Reload(context.Background(), "t1", c.file, c.layer, c.srid)

			if c.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}

func Test_SourceService_Reload_UnknownSource(t *testing.T) {
	svc, _ := setupSourceService(t, t.TempDir(), 1000, ".geojson")

	err := svc.Reload(context.Background(), "t1", "ghost.geojson", "", 0)

	var notFound *NotFoundError
	assert.ErrorAs(t, err, &notFound)
}
