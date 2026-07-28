package service

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"

	"gogeoserver/internal/config"
)

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

// tableImporter is everything SourceService needs from TableService —
// declared here, the consumer, rather than depending on the concrete
// *TableService: this is SourceService's actual dependency, not
// TableService's whole method set.
type tableImporter interface {
	LoadedSourceNames(ctx context.Context) (map[string]string, error)
	ListLayers(ctx context.Context, path string) ([]string, error)
	ImportFromFile(ctx context.Context, name, path, ext, layer string, srid int) (string, error)
	DetectCRS(ctx context.Context, path, layer string) (authName string, authCode int, err error)
	RemoveTable(ctx context.Context, tableId string) error
}

// SourceService scans workDir for loadable files and imports them into
// tables on demand. name is only ever accepted from the last Refresh/List
// cache (règle 15) — never a path built freely from the HTTP request, to
// eliminate any directory traversal risk. mu is held for the whole
// duration of Load's check-then-import, not just around the cache lookup,
// so two concurrent Load calls for the same source can't both pass the
// "not loaded yet" check before either has actually imported it.
type SourceService struct {
	cfg          config.SourcesConfig
	tableService tableImporter

	mu      sync.Mutex
	sources []Source
}

// NewSourceService builds a SourceService scanning cfg.WorkDir and
// importing through tableService.
func NewSourceService(cfg config.SourcesConfig, tableService tableImporter) *SourceService {
	return &SourceService{cfg: cfg, tableService: tableService}
}

// MaxUploadBytes returns the largest file Upload will accept, used by the
// router to size the request body limit on /sources/upload.
func (s *SourceService) MaxUploadBytes() int64 {
	return s.cfg.MaxSizeBytes
}

// Refresh rescans workDir and replaces the in-memory cache. Files beyond
// MaxSizeBytes or with a non-whitelisted extension are silently excluded,
// not reported as an error.
func (s *SourceService) Refresh(ctx context.Context) ([]Source, error) {
	entries, err := os.ReadDir(s.cfg.WorkDir)
	if err != nil {
		return nil, fmt.Errorf("reading workDir: %w", err)
	}

	loaded, err := s.tableService.LoadedSourceNames(ctx)
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
		if format, ok := lookupSourceFormat(ext); ok && format.MultiLayer {
			source.Layers = s.listLoadedLayers(ctx, entry.Name(), loaded)
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
func (s *SourceService) listLoadedLayers(ctx context.Context, name string, loaded map[string]string) []SourceLayer {
	path := filepath.Join(s.cfg.WorkDir, name)
	layerNames, err := s.tableService.ListLayers(ctx, path)
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

// List returns a copy of the cache from the last Refresh — it never
// rescans the disk itself, so an implicit scan per request doesn't defeat
// the intent of an explicit refresh or cost an os.ReadDir per call. A copy,
// not the live internal slice, so a caller can never mutate cache state
// out from under Load.
func (s *SourceService) List() []Source {
	s.mu.Lock()
	defer s.mu.Unlock()
	return cloneSources(s.sources)
}

func cloneSources(sources []Source) []Source {
	out := make([]Source, len(sources))
	for i, src := range sources {
		out[i] = src
		if src.Layers != nil {
			out[i].Layers = append([]SourceLayer(nil), src.Layers...)
		}
	}
	return out
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
// loaded as a whole. For any other source, layer must be empty. srid pins
// the source CRS explicitly (GEO-1); pass 0 to detect it from the file.
// Returns the newly created tableId. Held under mu for its whole duration
// (see SourceService doc) — the on-success update of the cached entry
// below is what lets an immediate second Load, with no Refresh in
// between, see the source as loaded instead of importing it again.
func (s *SourceService) Load(ctx context.Context, name, layer string, srid int) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var found *Source
	for i := range s.sources {
		if s.sources[i].Name == name {
			found = &s.sources[i]
			break
		}
	}
	if found == nil {
		return "", &NotFoundError{fmt.Sprintf("unknown source %q — call /sources/refresh first", name)}
	}

	format, known := lookupSourceFormat(found.Ext)
	if !known || !format.MultiLayer {
		if layer != "" {
			return "", &ValidationError{fmt.Sprintf("source %q does not support layer selection", name)}
		}
		if found.Loaded {
			return "", &ConflictError{fmt.Sprintf("source %q already loaded (table %s)", name, found.TableId)}
		}
		path := filepath.Join(s.cfg.WorkDir, found.Name)
		tableId, err := s.tableService.ImportFromFile(ctx, found.Name, path, found.Ext, "", srid)
		if err != nil {
			return "", err
		}
		found.Loaded = true
		found.TableId = tableId
		return tableId, nil
	}

	if layer == "" {
		return "", &ValidationError{fmt.Sprintf("source %q has multiple layers — specify one", name)}
	}
	var foundLayer *SourceLayer
	for i := range found.Layers {
		if found.Layers[i].Name == layer {
			foundLayer = &found.Layers[i]
			break
		}
	}
	if foundLayer == nil {
		return "", &NotFoundError{fmt.Sprintf("unknown layer %q for source %q — call /sources/refresh first", layer, name)}
	}
	if foundLayer.Loaded {
		return "", &ConflictError{fmt.Sprintf("layer %q of source %q already loaded (table %s)", layer, name, foundLayer.TableId)}
	}

	path := filepath.Join(s.cfg.WorkDir, found.Name)
	tableName := found.Name + layerSeparator + layer
	tableId, err := s.tableService.ImportFromFile(ctx, tableName, path, found.Ext, layer, srid)
	if err != nil {
		return "", err
	}
	foundLayer.Loaded = true
	foundLayer.TableId = tableId
	return tableId, nil
}

// sourcePath resolves name (and, for a multi-layer source, layer) against
// the cache exactly like Load does (règle 15 — name is never turned into a
// path without this check), without mutating any loaded state.
func (s *SourceService) sourcePath(name, layer string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var found *Source
	for i := range s.sources {
		if s.sources[i].Name == name {
			found = &s.sources[i]
			break
		}
	}
	if found == nil {
		return "", &NotFoundError{fmt.Sprintf("unknown source %q — call /sources/refresh first", name)}
	}

	if format, known := lookupSourceFormat(found.Ext); known && format.MultiLayer && layer != "" {
		found := slices.ContainsFunc(found.Layers, func(l SourceLayer) bool { return l.Name == layer })
		if !found {
			return "", &NotFoundError{fmt.Sprintf("unknown layer %q for source %q — call /sources/refresh first", layer, name)}
		}
	}

	return filepath.Join(s.cfg.WorkDir, name), nil
}

// DetectCRS reports the authority/code of a source's declared CRS (0 if
// undetectable), so a caller can preview it before Load — the "editable
// EPSG" flow (GEO-1). layer selects one layer of a multi-layer source.
func (s *SourceService) DetectCRS(ctx context.Context, name, layer string) (authName string, authCode int, err error) {
	path, err := s.sourcePath(name, layer)
	if err != nil {
		return "", 0, err
	}
	return s.tableService.DetectCRS(ctx, path, layer)
}

// Delete removes the table(s) loaded from name and drops its cache entry —
// the source file itself in workDir is never touched, only its loaded
// table(s) (deleting a user's uploaded file is out of scope; a caller who
// wants that must remove it from workDir directly). name is validated
// against the cache (règle 15), never an unvalidated path. The cache entry
// is only spliced out after every table removal has succeeded, so a
// failure midway (e.g. one layer's RemoveTable erroring after another's
// already succeeded) leaves the source visible for a retry instead of
// silently vanishing from the cache (NEW-BUG-6).
func (s *SourceService) Delete(ctx context.Context, name string) error {
	s.mu.Lock()
	index := -1
	for i := range s.sources {
		if s.sources[i].Name == name {
			index = i
			break
		}
	}
	if index == -1 {
		s.mu.Unlock()
		return &NotFoundError{fmt.Sprintf("unknown source %q — call /sources/refresh first", name)}
	}
	found := s.sources[index]
	s.mu.Unlock()

	if found.Loaded && found.TableId != "" {
		if err := s.tableService.RemoveTable(ctx, found.TableId); err != nil {
			return err
		}
		found.Loaded = false
		found.TableId = ""
	}
	for i, layer := range found.Layers {
		if layer.Loaded && layer.TableId != "" {
			if err := s.tableService.RemoveTable(ctx, layer.TableId); err != nil {
				return err
			}
			found.Layers[i].Loaded = false
			found.Layers[i].TableId = ""
		}
	}

	s.mu.Lock()
	for i := range s.sources {
		if s.sources[i].Name == name {
			s.sources = slices.Delete(s.sources, i, i+1)
			break
		}
	}
	s.mu.Unlock()
	return nil
}
