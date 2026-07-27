package service

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"

	"gogeoserver/internal/config"
)

// gpkgExt is the only extension currently treated as a multi-layer format:
// its layers are enumerated in Source.Layers, and each loads into its own
// table (a GeoPackage is never "loaded" as a whole).
const gpkgExt = ".gpkg"

// layerSeparator joins a source file name and layer name into the composite
// table name each loaded layer is registered under (see Load), so a single
// GeoPackage can back several independently loaded tables.
const layerSeparator = "::"

// Source describes a file found in workDir, whether or not it has been
// loaded into a table yet.
type Source struct {
	Name      string        `json:"name"`
	Ext       string        `json:"ext"`
	SizeBytes int64         `json:"sizeBytes"`
	ModTime   time.Time     `json:"modTime"`
	Loaded    bool          `json:"loaded"`
	TableId   string        `json:"tableId,omitempty"`
	Layers    []SourceLayer `json:"layers,omitempty"`
}

// SourceLayer describes one layer of a multi-layer source (e.g. a
// GeoPackage feature table), tracked independently since each layer loads
// into its own table.
type SourceLayer struct {
	Name    string `json:"name"`
	Loaded  bool   `json:"loaded"`
	TableId string `json:"tableId,omitempty"`
}

// SourceService scans workDir for loadable files and imports them into
// tables on demand. name is only ever accepted from the last Refresh/List
// cache (règle 15) — never a path built freely from the HTTP request, to
// eliminate any directory traversal risk.
type SourceService struct {
	cfg          config.SourcesConfig
	tableService *TableService

	mu      sync.RWMutex
	sources []Source
}

// NewSourceService builds a SourceService scanning cfg.WorkDir and
// importing through tableService.
func NewSourceService(cfg config.SourcesConfig, tableService *TableService) *SourceService {
	return &SourceService{cfg: cfg, tableService: tableService}
}

// Refresh rescans workDir and replaces the in-memory cache. Files beyond
// MaxSizeBytes or with a non-whitelisted extension are silently excluded,
// not reported as an error.
func (s *SourceService) Refresh() ([]Source, error) {
	entries, err := os.ReadDir(s.cfg.WorkDir)
	if err != nil {
		return nil, fmt.Errorf("reading workDir: %w", err)
	}

	loaded, err := s.tableService.LoadedSourceNames()
	if err != nil {
		return nil, err
	}

	sources := []Source{}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := filepath.Ext(entry.Name())
		if !slices.Contains(s.cfg.AllowedExtensions, ext) {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if info.Size() > s.cfg.MaxSizeBytes {
			continue
		}

		source := Source{
			Name:      entry.Name(),
			Ext:       ext,
			SizeBytes: info.Size(),
			ModTime:   info.ModTime(),
		}
		if ext == gpkgExt {
			source.Layers = s.listLoadedLayers(entry.Name(), loaded)
		} else {
			source.TableId, source.Loaded = loaded[entry.Name()]
		}
		sources = append(sources, source)
	}

	s.mu.Lock()
	s.sources = sources
	s.mu.Unlock()
	return sources, nil
}

// listLoadedLayers enumerates name's layers and cross-references each
// against loaded (keyed the same way Load names a loaded layer's table), so
// a multi-layer source's loaded state is tracked per layer. If the file's
// layers can't be read (e.g. corrupt), it is still listed as a source, just
// without layer info, rather than failing the whole Refresh.
func (s *SourceService) listLoadedLayers(name string, loaded map[string]string) []SourceLayer {
	path := filepath.Join(s.cfg.WorkDir, name)
	layerNames, err := s.tableService.ListLayers(path)
	if err != nil {
		return nil
	}

	layers := make([]SourceLayer, len(layerNames))
	for i, layerName := range layerNames {
		tableId, isLoaded := loaded[name+layerSeparator+layerName]
		layers[i] = SourceLayer{Name: layerName, Loaded: isLoaded, TableId: tableId}
	}
	return layers
}

// List returns the cache from the last Refresh — it never rescans the
// disk itself, so an implicit scan per request doesn't defeat the intent
// of an explicit refresh or cost an os.ReadDir per call.
func (s *SourceService) List() []Source {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.sources == nil {
		return []Source{}
	}
	return s.sources
}

// Upload writes content to workDir under filename, validated against the
// same extension whitelist and size limit as Refresh, and rejected outright
// if a file with that name already exists — callers wanting to replace a
// source must remove it first, never silently overwritten. filename is
// sanitized to its base name to eliminate directory traversal (règle 15).
// Returns the sanitized name actually written, for chaining into Load.
func (s *SourceService) Upload(filename string, content io.Reader) (string, error) {
	filename = filepath.Base(filename)
	ext := filepath.Ext(filename)
	if !slices.Contains(s.cfg.AllowedExtensions, ext) {
		return "", fmt.Errorf("extension not allowed: %q", ext)
	}

	path := filepath.Join(s.cfg.WorkDir, filename)
	out, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		if os.IsExist(err) {
			return "", fmt.Errorf("a file named %q already exists in workDir", filename)
		}
		return "", fmt.Errorf("creating file: %w", err)
	}
	defer out.Close()

	written, err := io.Copy(out, io.LimitReader(content, s.cfg.MaxSizeBytes+1))
	if err != nil {
		os.Remove(path)
		return "", fmt.Errorf("writing file: %w", err)
	}
	if written > s.cfg.MaxSizeBytes {
		os.Remove(path)
		return "", fmt.Errorf("file exceeds maxSizeBytes (%d bytes)", s.cfg.MaxSizeBytes)
	}
	return filename, nil
}

// Load imports the source named name, which must match an entry in the
// current cache (règle 15). For a multi-layer (.gpkg) source, layer must
// name one of that source's cached layers — the file itself is never
// loaded as a whole. For any other source, layer must be empty. Returns
// the newly created tableId.
func (s *SourceService) Load(name, layer string) (string, error) {
	s.mu.RLock()
	var found *Source
	for i := range s.sources {
		if s.sources[i].Name == name {
			found = &s.sources[i]
			break
		}
	}
	s.mu.RUnlock()

	if found == nil {
		return "", fmt.Errorf("unknown source %q — call /sources/refresh first", name)
	}

	if found.Ext != gpkgExt {
		if layer != "" {
			return "", fmt.Errorf("source %q does not support layer selection", name)
		}
		if found.Loaded {
			return "", fmt.Errorf("source %q already loaded (table %s)", name, found.TableId)
		}
		path := filepath.Join(s.cfg.WorkDir, found.Name)
		return s.tableService.ImportFromFile(found.Name, path, found.Ext, "")
	}

	if layer == "" {
		return "", fmt.Errorf("source %q has multiple layers — specify one", name)
	}
	var foundLayer *SourceLayer
	for i := range found.Layers {
		if found.Layers[i].Name == layer {
			foundLayer = &found.Layers[i]
			break
		}
	}
	if foundLayer == nil {
		return "", fmt.Errorf("unknown layer %q for source %q — call /sources/refresh first", layer, name)
	}
	if foundLayer.Loaded {
		return "", fmt.Errorf("layer %q of source %q already loaded (table %s)", layer, name, foundLayer.TableId)
	}

	path := filepath.Join(s.cfg.WorkDir, found.Name)
	tableName := found.Name + layerSeparator + layer
	return s.tableService.ImportFromFile(tableName, path, found.Ext, layer)
}
