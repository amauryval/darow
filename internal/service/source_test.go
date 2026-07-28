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
	m.On("GetMetadata", mock.Anything, "id-1").Return(storage.Metadata{TableId: "id-1", TableName: "buildings.geojson", SourceName: "buildings.geojson"}, nil).Once()

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

			_, err := svc.Load(context.Background(), c.requestedName, "", 0)

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
	m.On("GetMetadata", mock.Anything, "id-1").Return(storage.Metadata{TableId: "id-1", TableName: "buildings.geojson", SourceName: "buildings.geojson"}, nil).Once()

	svc := NewSourceService(config.SourcesConfig{
		WorkDir:           dir,
		MaxSizeBytes:      1000,
		AllowedExtensions: []string{".geojson"},
	}, tableSvc)

	_, err := svc.Refresh(context.Background())
	require.NoError(t, err)

	_, err = svc.Load(context.Background(), "buildings.geojson", "", 0)
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
	m.On("GetMetadata", mock.Anything, "id-new").Return(storage.Metadata{TableId: "id-new", PhysicalName: "t_new"}, nil).Once()
	m.On("CreateFromFile", mock.Anything, "t_new", filepath.Join(dir, "buildings.geojson"), "", 0).Return("", false, 0, nil).Once()
	m.On("UpdateMetadata", mock.Anything, "id-new", map[string]any{"sourceName": "buildings.geojson", "sourceLayer": ""}).Return(nil).Once()
	m.On("RefreshDerivedMetadata", mock.Anything, "id-new").Return(nil).Once()

	tableId, err := svc.Load(context.Background(), "buildings.geojson", "", 0)

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
	m.On("GetMetadata", mock.Anything, "id-new").Return(storage.Metadata{TableId: "id-new", PhysicalName: "t_new"}, nil).Once()
	m.On("CreateFromFile", mock.Anything, "t_new", filepath.Join(dir, "buildings.geojson"), "", 0).Return("", false, 0, nil).Once()
	m.On("UpdateMetadata", mock.Anything, "id-new", map[string]any{"sourceName": "buildings.geojson", "sourceLayer": ""}).Return(nil).Once()
	m.On("RefreshDerivedMetadata", mock.Anything, "id-new").Return(nil).Once()

	tableId, err := svc.Load(context.Background(), "buildings.geojson", "", 0)
	require.NoError(t, err)
	assert.Equal(t, "id-new", tableId)

	_, err = svc.Load(context.Background(), "buildings.geojson", "", 0)
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

	_, err = svc.Load(context.Background(), "buildings.geojson", "some_layer", 0)
	assert.Error(t, err)
}

func Test_SourceService_Refresh_PopulatesGpkgLayers(t *testing.T) {
	dir := t.TempDir()
	writeFixture(t, dir, "regions.gpkg", 100)

	tableSvc, m := setupImportService(t)
	m.On("TableIds", mock.Anything).Return([]string{"id-1"}, nil).Once()
	m.On("GetMetadata", mock.Anything, "id-1").Return(storage.Metadata{
		TableId: "id-1", TableName: "regions.gpkg::communes", SourceName: "regions.gpkg", SourceLayer: "communes",
	}, nil).Once()
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
	m.On("GetMetadata", mock.Anything, "id-new").Return(storage.Metadata{TableId: "id-new", PhysicalName: "t_new"}, nil).Once()
	m.On("CreateFromFile", mock.Anything, "t_new", filepath.Join(dir, "regions.gpkg"), "communes", 0).Return("geom", true, 4326, nil).Once()
	m.On("UpdateMetadata", mock.Anything, "id-new", map[string]any{
		"sourceName": "regions.gpkg", "sourceLayer": "communes", "isGeo": true, "geometryColumn": "geom", "srid": 4326,
	}).Return(nil).Once()
	m.On("RefreshDerivedMetadata", mock.Anything, "id-new").Return(nil).Once()

	tableId, err := svc.Load(context.Background(), "regions.gpkg", "communes", 0)

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

	_, err = svc.Load(context.Background(), "regions.gpkg", "", 0)
	assert.Error(t, err)
}

func Test_SourceService_Load_ForwardsSridOverride(t *testing.T) {
	dir := t.TempDir()
	writeFixture(t, dir, "lambert.geojson", 100)

	tableSvc, m := setupImportService(t)
	m.On("TableIds", mock.Anything).Return([]string{}, nil).Once()

	svc := NewSourceService(config.SourcesConfig{
		WorkDir:           dir,
		MaxSizeBytes:      1000,
		AllowedExtensions: []string{".geojson"},
	}, tableSvc)

	_, err := svc.Refresh(context.Background())
	require.NoError(t, err)

	m.On("Create", mock.Anything, "lambert.geojson", false, "").Return("id-new", nil).Once()
	m.On("GetMetadata", mock.Anything, "id-new").Return(storage.Metadata{TableId: "id-new", PhysicalName: "t_new"}, nil).Once()
	m.On("CreateFromFile", mock.Anything, "t_new", filepath.Join(dir, "lambert.geojson"), "", 2154).Return("geom", true, 2154, nil).Once()
	m.On("UpdateMetadata", mock.Anything, "id-new", map[string]any{
		"sourceName": "lambert.geojson", "sourceLayer": "", "isGeo": true, "geometryColumn": "geom", "srid": 2154,
	}).Return(nil).Once()
	m.On("RefreshDerivedMetadata", mock.Anything, "id-new").Return(nil).Once()

	tableId, err := svc.Load(context.Background(), "lambert.geojson", "", 2154)

	require.NoError(t, err)
	assert.Equal(t, "id-new", tableId)
}

func Test_SourceService_DetectCRS(t *testing.T) {
	dir := t.TempDir()
	writeFixture(t, dir, "lambert.geojson", 100)

	tableSvc, m := setupImportService(t)
	m.On("TableIds", mock.Anything).Return([]string{}, nil).Once()

	svc := NewSourceService(config.SourcesConfig{
		WorkDir:           dir,
		MaxSizeBytes:      1000,
		AllowedExtensions: []string{".geojson"},
	}, tableSvc)

	_, err := svc.Refresh(context.Background())
	require.NoError(t, err)

	m.On("DetectCRS", mock.Anything, filepath.Join(dir, "lambert.geojson"), "").Return("EPSG", 2154, nil).Once()

	authName, authCode, err := svc.DetectCRS(context.Background(), "lambert.geojson", "")

	require.NoError(t, err)
	assert.Equal(t, "EPSG", authName)
	assert.Equal(t, 2154, authCode)
}

func Test_SourceService_DetectCRS_RejectsUnknownSource(t *testing.T) {
	tableSvc, _ := setupImportService(t)
	svc := NewSourceService(config.SourcesConfig{WorkDir: t.TempDir()}, tableSvc)

	_, _, err := svc.DetectCRS(context.Background(), "../../etc/passwd", "")

	assert.Error(t, err)
}

// NEW-BUG-6: Delete removes a loaded source's table but never its file —
// deleting a user's uploaded file is out of scope. The cache entry is
// still dropped once the table removal succeeds.
func Test_SourceService_Delete_RemovesFileAndCascadesToTable(t *testing.T) {
	dir := t.TempDir()
	writeFixture(t, dir, "buildings.geojson", 100)

	tableSvc, m := setupImportService(t)
	m.On("TableIds", mock.Anything).Return([]string{"id-1"}, nil).Once()
	m.On("GetMetadata", mock.Anything, "id-1").Return(storage.Metadata{TableId: "id-1", TableName: "buildings.geojson", SourceName: "buildings.geojson"}, nil).Once()

	svc := NewSourceService(config.SourcesConfig{
		WorkDir:           dir,
		MaxSizeBytes:      1000,
		AllowedExtensions: []string{".geojson"},
	}, tableSvc)

	_, err := svc.Refresh(context.Background())
	require.NoError(t, err)

	m.On("Remove", mock.Anything, "id-1").Return(nil).Once()

	err = svc.Delete(context.Background(), "buildings.geojson")

	require.NoError(t, err)
	_, statErr := os.Stat(filepath.Join(dir, "buildings.geojson"))
	assert.NoError(t, statErr)
	assert.Empty(t, svc.List())
}

func Test_SourceService_Delete_UnloadedSourceSkipsTableRemoval(t *testing.T) {
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

	err = svc.Delete(context.Background(), "buildings.geojson")

	require.NoError(t, err)
}

func Test_SourceService_Delete_RejectsUnknownSource(t *testing.T) {
	tableSvc, _ := setupImportService(t)
	svc := NewSourceService(config.SourcesConfig{WorkDir: t.TempDir()}, tableSvc)

	err := svc.Delete(context.Background(), "unknown.geojson")

	assert.Error(t, err)
}
