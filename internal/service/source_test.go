package service

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"gogeoserver/internal/config"
	"gogeoserver/internal/storage"
)

func writeFixture(t *testing.T, dir, name string, size int) {
	path := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(path, make([]byte, size), 0o644))
}

func Test_SourceService_Refresh(t *testing.T) {
	dir := t.TempDir()
	writeFixture(t, dir, "ok.geojson", 100)
	writeFixture(t, dir, "too_big.geojson", 200)
	writeFixture(t, dir, "ignored.txt", 100)

	tableSvc, m := setupImportService(t)
	m.On("TableIds", mock.Anything).Return([]string{}, nil).Once()

	svc := NewSourceService(config.SourcesConfig{
		WorkDir:           dir,
		MaxSizeBytes:      150,
		AllowedExtensions: []string{".geojson"},
	}, tableSvc)

	sources, err := svc.Refresh(context.Background())

	require.NoError(t, err)
	require.Len(t, sources, 1)
	assert.Equal(t, "ok.geojson", sources[0].Name)
}

func Test_SourceService_Refresh_MarksLoadedSources(t *testing.T) {
	dir := t.TempDir()
	writeFixture(t, dir, "buildings.geojson", 100)

	tableSvc, m := setupImportService(t)
	m.On("TableIds", mock.Anything).Return([]string{"id-1"}, nil).Once()
	m.On("GetMetadata", mock.Anything, "id-1").Return(&storage.Metadata{TableId: "id-1", TableName: "buildings.geojson"}, nil).Once()

	svc := NewSourceService(config.SourcesConfig{
		WorkDir:           dir,
		MaxSizeBytes:      1000,
		AllowedExtensions: []string{".geojson"},
	}, tableSvc)

	sources, err := svc.Refresh(context.Background())

	require.NoError(t, err)
	require.Len(t, sources, 1)
	assert.True(t, sources[0].Loaded)
	assert.Equal(t, "id-1", sources[0].TableId)
}

func Test_SourceService_Load(t *testing.T) {
	cases := []struct {
		name          string
		requestedName string
		wantErr       bool
	}{
		{name: "rejects name not in cache", requestedName: "../../etc/passwd", wantErr: true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			tableSvc, _ := setupImportService(t)
			svc := NewSourceService(config.SourcesConfig{WorkDir: t.TempDir()}, tableSvc)

			_, err := svc.Load(context.Background(), c.requestedName, "")

			if c.wantErr {
				assert.Error(t, err)
				return
			}
		})
	}
}

func Test_SourceService_Upload(t *testing.T) {
	cases := []struct {
		name        string
		filename    string
		content     string
		preexisting bool
		maxSize     int64
		wantErr     bool
		wantSaved   string
	}{
		{
			name:      "writes file and sanitizes traversal",
			filename:  "../../etc/evil.geojson",
			content:   `{"type":"FeatureCollection","features":[]}`,
			maxSize:   1000,
			wantSaved: "evil.geojson",
		},
		{
			name:     "rejects disallowed extension",
			filename: "malware.exe",
			content:  "x",
			maxSize:  1000,
			wantErr:  true,
		},
		{
			name:        "rejects already existing file",
			filename:    "buildings.geojson",
			content:     "x",
			preexisting: true,
			maxSize:     1000,
			wantErr:     true,
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

			tableSvc, _ := setupImportService(t)
			svc := NewSourceService(config.SourcesConfig{
				WorkDir:           dir,
				MaxSizeBytes:      c.maxSize,
				AllowedExtensions: []string{".geojson"},
			}, tableSvc)

			saved, err := svc.Upload(c.filename, strings.NewReader(c.content))

			if c.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, c.wantSaved, saved)

			data, err := os.ReadFile(filepath.Join(dir, saved))
			require.NoError(t, err)
			assert.Equal(t, c.content, string(data))
		})
	}
}

func Test_SourceService_Load_RejectsAlreadyLoaded(t *testing.T) {
	dir := t.TempDir()
	writeFixture(t, dir, "buildings.geojson", 100)

	tableSvc, m := setupImportService(t)
	m.On("TableIds", mock.Anything).Return([]string{"id-1"}, nil).Once()
	m.On("GetMetadata", mock.Anything, "id-1").Return(&storage.Metadata{TableId: "id-1", TableName: "buildings.geojson"}, nil).Once()

	svc := NewSourceService(config.SourcesConfig{
		WorkDir:           dir,
		MaxSizeBytes:      1000,
		AllowedExtensions: []string{".geojson"},
	}, tableSvc)

	_, err := svc.Refresh(context.Background())
	require.NoError(t, err)

	_, err = svc.Load(context.Background(), "buildings.geojson", "")
	assert.Error(t, err)
}

func Test_SourceService_Load_ImportsUnloadedSource(t *testing.T) {
	dir := t.TempDir()
	writeFixture(t, dir, "buildings.geojson", 100)

	tableSvc, m := setupImportService(t)
	m.On("TableIds", mock.Anything).Return([]string{}, nil).Once()

	svc := NewSourceService(config.SourcesConfig{
		WorkDir:           dir,
		MaxSizeBytes:      1000,
		AllowedExtensions: []string{".geojson"},
	}, tableSvc)

	_, err := svc.Refresh(context.Background())
	require.NoError(t, err)

	m.On("Create", mock.Anything, "buildings.geojson", false, "").Return("id-new", nil).Once()
	m.On("GetMetadata", mock.Anything, "id-new").Return(&storage.Metadata{TableId: "id-new", PhysicalName: "t_new"}, nil).Once()
	m.On("CreateFromFile", mock.Anything, "t_new", filepath.Join(dir, "buildings.geojson"), "").Return("", false, nil).Once()
	m.On("RefreshDerivedMetadata", mock.Anything, "id-new").Return(nil).Once()

	tableId, err := svc.Load(context.Background(), "buildings.geojson", "")

	require.NoError(t, err)
	assert.Equal(t, "id-new", tableId)
}

// Without this, a second Load for the same source — with no Refresh in
// between — would import it again, since the in-memory cache never learns
// about the first Load's success (BUG-3).
func Test_SourceService_Load_RejectsSecondLoadWithoutRefresh(t *testing.T) {
	dir := t.TempDir()
	writeFixture(t, dir, "buildings.geojson", 100)

	tableSvc, m := setupImportService(t)
	m.On("TableIds", mock.Anything).Return([]string{}, nil).Once()

	svc := NewSourceService(config.SourcesConfig{
		WorkDir:           dir,
		MaxSizeBytes:      1000,
		AllowedExtensions: []string{".geojson"},
	}, tableSvc)

	_, err := svc.Refresh(context.Background())
	require.NoError(t, err)

	m.On("Create", mock.Anything, "buildings.geojson", false, "").Return("id-new", nil).Once()
	m.On("GetMetadata", mock.Anything, "id-new").Return(&storage.Metadata{TableId: "id-new", PhysicalName: "t_new"}, nil).Once()
	m.On("CreateFromFile", mock.Anything, "t_new", filepath.Join(dir, "buildings.geojson"), "").Return("", false, nil).Once()
	m.On("RefreshDerivedMetadata", mock.Anything, "id-new").Return(nil).Once()

	tableId, err := svc.Load(context.Background(), "buildings.geojson", "")
	require.NoError(t, err)
	assert.Equal(t, "id-new", tableId)

	_, err = svc.Load(context.Background(), "buildings.geojson", "")
	assert.ErrorContains(t, err, "already loaded")
}

// List must hand out an independent snapshot: mutating the returned slice
// (or a Source's Layers within it) must never reach back into the cache
// Load reads from.
func Test_SourceService_List_ReturnsIndependentCopy(t *testing.T) {
	dir := t.TempDir()
	writeFixture(t, dir, "buildings.geojson", 100)

	tableSvc, m := setupImportService(t)
	m.On("TableIds", mock.Anything).Return([]string{}, nil).Once()

	svc := NewSourceService(config.SourcesConfig{
		WorkDir:           dir,
		MaxSizeBytes:      1000,
		AllowedExtensions: []string{".geojson"},
	}, tableSvc)

	_, err := svc.Refresh(context.Background())
	require.NoError(t, err)

	sources := svc.List()
	require.Len(t, sources, 1)
	sources[0].Loaded = true
	sources[0].TableId = "tampered"

	assert.False(t, svc.List()[0].Loaded)
	assert.Empty(t, svc.List()[0].TableId)
}

func Test_SourceService_Load_RejectsLayerForNonGpkg(t *testing.T) {
	dir := t.TempDir()
	writeFixture(t, dir, "buildings.geojson", 100)

	tableSvc, m := setupImportService(t)
	m.On("TableIds", mock.Anything).Return([]string{}, nil).Once()

	svc := NewSourceService(config.SourcesConfig{
		WorkDir:           dir,
		MaxSizeBytes:      1000,
		AllowedExtensions: []string{".geojson"},
	}, tableSvc)

	_, err := svc.Refresh(context.Background())
	require.NoError(t, err)

	_, err = svc.Load(context.Background(), "buildings.geojson", "some_layer")
	assert.Error(t, err)
}

func Test_SourceService_Refresh_PopulatesGpkgLayers(t *testing.T) {
	dir := t.TempDir()
	writeFixture(t, dir, "regions.gpkg", 100)

	tableSvc, m := setupImportService(t)
	m.On("TableIds", mock.Anything).Return([]string{"id-1"}, nil).Once()
	m.On("GetMetadata", mock.Anything, "id-1").Return(&storage.Metadata{TableId: "id-1", TableName: "regions.gpkg::communes"}, nil).Once()
	m.On("ListLayers", mock.Anything, filepath.Join(dir, "regions.gpkg")).Return([]string{"communes", "departements"}, nil).Once()

	svc := NewSourceService(config.SourcesConfig{
		WorkDir:           dir,
		MaxSizeBytes:      1000,
		AllowedExtensions: []string{".gpkg"},
	}, tableSvc)

	sources, err := svc.Refresh(context.Background())

	require.NoError(t, err)
	require.Len(t, sources, 1)
	assert.False(t, sources[0].Loaded)
	assert.Empty(t, sources[0].TableId)
	require.Len(t, sources[0].Layers, 2)
	assert.Equal(t, SourceLayer{Name: "communes", Loaded: true, TableId: "id-1"}, sources[0].Layers[0])
	assert.Equal(t, SourceLayer{Name: "departements", Loaded: false}, sources[0].Layers[1])
}

func Test_SourceService_Load_GpkgLayer(t *testing.T) {
	dir := t.TempDir()
	writeFixture(t, dir, "regions.gpkg", 100)

	tableSvc, m := setupImportService(t)
	m.On("TableIds", mock.Anything).Return([]string{}, nil).Once()
	m.On("ListLayers", mock.Anything, filepath.Join(dir, "regions.gpkg")).Return([]string{"communes", "departements"}, nil).Once()

	svc := NewSourceService(config.SourcesConfig{
		WorkDir:           dir,
		MaxSizeBytes:      1000,
		AllowedExtensions: []string{".gpkg"},
	}, tableSvc)

	_, err := svc.Refresh(context.Background())
	require.NoError(t, err)

	m.On("Create", mock.Anything, "regions.gpkg::communes", false, "").Return("id-new", nil).Once()
	m.On("GetMetadata", mock.Anything, "id-new").Return(&storage.Metadata{TableId: "id-new", PhysicalName: "t_new"}, nil).Once()
	m.On("CreateFromFile", mock.Anything, "t_new", filepath.Join(dir, "regions.gpkg"), "communes").Return("geom", true, nil).Once()
	m.On("UpdateMetadata", mock.Anything, "id-new", map[string]any{"isGeo": true, "geometryColumn": "geom"}).Return(nil).Once()
	m.On("RefreshDerivedMetadata", mock.Anything, "id-new").Return(nil).Once()

	tableId, err := svc.Load(context.Background(), "regions.gpkg", "communes")

	require.NoError(t, err)
	assert.Equal(t, "id-new", tableId)
}

func Test_SourceService_Load_GpkgRequiresLayer(t *testing.T) {
	dir := t.TempDir()
	writeFixture(t, dir, "regions.gpkg", 100)

	tableSvc, m := setupImportService(t)
	m.On("TableIds", mock.Anything).Return([]string{}, nil).Once()
	m.On("ListLayers", mock.Anything, filepath.Join(dir, "regions.gpkg")).Return([]string{"communes"}, nil).Once()

	svc := NewSourceService(config.SourcesConfig{
		WorkDir:           dir,
		MaxSizeBytes:      1000,
		AllowedExtensions: []string{".gpkg"},
	}, tableSvc)

	_, err := svc.Refresh(context.Background())
	require.NoError(t, err)

	_, err = svc.Load(context.Background(), "regions.gpkg", "")
	assert.Error(t, err)
}
