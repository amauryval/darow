package service

import (
	"cmp"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"

	"darow/internal/config"
	"darow/internal/storage"
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
	SourceHasGeometry(ctx context.Context, path, layer string) (bool, error)
	RemoveTable(ctx context.Context, tableId string) error
}

// SourceService scans workDir for loadable files and imports them into
// tables on demand. name is only ever accepted from the last Refresh/List
// cache — never a path built freely from the HTTP request, to
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
		return nil, NewDisplayError("unable to scan the sources directory", err)
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

// sourceSortFields maps a sort field's public name to a comparator over two
// Sources — an allowlist, mirroring metadataSortColumns/rowsOrderBy's role
// for the SQL-backed listings, since Source has no query to validate an
// arbitrary column against.
var sourceSortFields = map[string]func(a, b Source) int{
	"name":      func(a, b Source) int { return cmp.Compare(a.Name, b.Name) },
	"sizeBytes": func(a, b Source) int { return cmp.Compare(a.SizeBytes, b.SizeBytes) },
	"modTime":   func(a, b Source) int { return a.ModTime.Compare(b.ModTime) },
	"loaded":    func(a, b Source) int { return cmp.Compare(boolToInt(a.Loaded), boolToInt(b.Loaded)) },
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// sortSources sorts sources in place by sort (multi-column, each field
// validated against sourceSortFields), stable so equal keys keep their
// existing relative order.
func sortSources(sources []Source, sort []storage.SortField) error {
	comparators := make([]func(a, b Source) int, len(sort))
	for i, f := range sort {
		cmpFn, ok := sourceSortFields[f.Column]
		if !ok {
			return &ValidationError{fmt.Sprintf("invalid sort column: %q", f.Column)}
		}
		if f.Desc {
			inner := cmpFn
			cmpFn = func(a, b Source) int { return -inner(a, b) }
		}
		comparators[i] = cmpFn
	}

	slices.SortStableFunc(sources, func(a, b Source) int {
		for _, cmpFn := range comparators {
			if c := cmpFn(a, b); c != 0 {
				return c
			}
		}
		return 0
	})
	return nil
}

// List returns one page (offset/limit over the top-level sources, a
// multi-layer source counting as one item regardless of its layer count)
// of the cache from the last Refresh, ordered by sort or by scan order when
// empty, plus the total count — it never rescans the disk itself, so an
// implicit scan per request doesn't defeat the intent of an explicit
// refresh or cost an os.ReadDir per call. Sorts a copy, not the live
// internal slice, so a caller can never mutate cache state out from under
// Load.
func (s *SourceService) List(sort []storage.SortField, offset, limit int) ([]Source, int, error) {
	s.mu.Lock()
	sources := cloneSources(s.sources)
	s.mu.Unlock()

	if err := sortSources(sources, sort); err != nil {
		return nil, 0, err
	}

	total := len(sources)
	start := min(max(offset, 0), total)
	end := min(max(start+limit, start), total)
	return sources[start:end], total, nil
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
// sanitized to its base name to eliminate directory traversal.
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
			return "", &ConflictError{fmt.Sprintf("a file named %q already exists in workDir", filename)}
		}
		return "", NewDisplayError(fmt.Sprintf("unable to create file %q in workDir", filename), err)
	}
	defer out.Close()

	written, err := io.Copy(out, io.LimitReader(content, s.cfg.MaxSizeBytes+1))
	if err != nil {
		os.Remove(path)
		return "", NewDisplayError(fmt.Sprintf("unable to write file %q", filename), err)
	}
	if written > s.cfg.MaxSizeBytes {
		os.Remove(path)
		return "", fmt.Errorf("file exceeds maxSizeBytes (%d bytes)", s.cfg.MaxSizeBytes)
	}
	return filename, nil
}

// Load imports the source named name, which must match an entry in the
// current cache. For a multi-layer (.gpkg) source, layer must
// name one of that source's cached layers — the file itself is never
// loaded as a whole. For any other source, layer must be empty. srid pins
// the source CRS explicitly; pass 0 to detect it from the file.
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

// UnmarkLoaded drops meta's loaded state from the cache, so a table deleted
// elsewhere (MetadataService.RemoveTable) stops being reported as loaded —
// registered onto core.NewServices' removed observer. Looked up by
// SourceName/SourceLayer, not TableId: those are what a removed table's
// metadata carries, and finding the entry by them needs no reverse scan
// keyed on a field the cache wasn't already tracking for that purpose. A
// no-op if meta has no source, or no entry matches it.
func (s *SourceService) UnmarkLoaded(meta storage.Metadata) {
	if meta.SourceName == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.sources {
		if s.sources[i].Name != meta.SourceName {
			continue
		}
		if meta.SourceLayer == "" {
			s.sources[i].Loaded = false
			s.sources[i].TableId = ""
			continue
		}
		for j := range s.sources[i].Layers {
			if s.sources[i].Layers[j].Name == meta.SourceLayer {
				s.sources[i].Layers[j].Loaded = false
				s.sources[i].Layers[j].TableId = ""
			}
		}
	}
}

// sourcePath resolves name (and, for a multi-layer source, layer) against
// the cache exactly like Load does — name is never turned into a path
// without this check — without mutating any loaded state.
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
// EPSG" flow. layer selects one layer of a multi-layer source. A format
// that isn't Introspectable (e.g. Parquet/GeoParquet, resolved instead at
// Load time) reports no CRS rather than calling into storage — its
// declared-CRS reader is never the same one ListLayers/DetectCRS use for
// every other format, and must never be reached with the wrong file type.
func (s *SourceService) DetectCRS(ctx context.Context, name, layer string) (authName string, authCode int, err error) {
	path, err := s.sourcePath(name, layer)
	if err != nil {
		return "", 0, err
	}
	if format, ok := lookupSourceFormat(filepath.Ext(name)); ok && !format.Introspectable {
		return "", 0, nil
	}
	authName, authCode, err = s.tableService.DetectCRS(ctx, path, layer)
	if err != nil {
		return "", 0, NewDisplayError(fmt.Sprintf("unable to detect the CRS of source %q", name), err)
	}
	return authName, authCode, nil
}

// SourceHasGeometry reports whether a source carries a geometry field at all —
// what tells a caller whether the CRS question applies to it. Every format is
// asked, Parquet included: the storage layer reads each one the way that
// format has to be read, exactly as CreateFromFile does, so a plain Parquet
// answers no rather than being assumed to carry a geometry.
func (s *SourceService) SourceHasGeometry(ctx context.Context, name, layer string) (bool, error) {
	path, err := s.sourcePath(name, layer)
	if err != nil {
		return false, err
	}
	hasGeometry, err := s.tableService.SourceHasGeometry(ctx, path, layer)
	if err != nil {
		return false, NewDisplayError(fmt.Sprintf("unable to read the geometry fields of source %q", name), err)
	}
	return hasGeometry, nil
}

// Delete removes the table(s) loaded from name and drops its cache entry —
// the source file itself in workDir is never touched, only its loaded
// table(s) (deleting a user's uploaded file is out of scope; a caller who
// wants that must remove it from workDir directly). name is validated
// against the cache, never an unvalidated path. The cache entry
// is only spliced out after every table removal has succeeded, so a
// failure midway (e.g. one layer's RemoveTable erroring after another's
// already succeeded) leaves the source visible for a retry instead of
// silently vanishing from the cache. found is read under mu and
// never mutated outside it: it is about to be deleted from the cache
// wholesale, so updating its Loaded/TableId fields on the way out would only
// race a concurrent List against the live entry's shared Layers slice for no
// observable benefit.
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
	}
	for _, layer := range found.Layers {
		if layer.Loaded && layer.TableId != "" {
			if err := s.tableService.RemoveTable(ctx, layer.TableId); err != nil {
				return err
			}
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
