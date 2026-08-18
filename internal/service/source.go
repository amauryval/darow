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

// layerSeparator joins a source file name and a layer name into the composite
// table name each loaded layer is registered under.
const layerSeparator = "::"

// Source describes a file found in workDir. It says nothing about the tables
// loaded from it: a loaded table holds its own copy of the data and lives on
// its own from then on.
type Source struct {
	Name      string    `json:"name"`
	Ext       string    `json:"ext"`
	SizeBytes int64     `json:"sizeBytes"`
	ModTime   time.Time `json:"modTime"`
	Layers    []string  `json:"layers,omitempty"`
}

// tableImporter is everything SourceService needs from ImportService.
type tableImporter interface {
	ListLayers(ctx context.Context, path string) ([]string, error)
	ImportFromFile(ctx context.Context, name, tableName, path, ext, layer string, srid int) (string, error)
	ReloadFromFile(ctx context.Context, tableId, name, path, layer string, srid int) error
	DetectCRS(ctx context.Context, path, layer string) (authName string, authCode int, err error)
	SourceHasGeometry(ctx context.Context, path, layer string) (bool, error)
}

// SourceService scans workDir for loadable files and imports them into tables
// on demand. A name is only ever accepted from the last Refresh's cache,
// never built from the request, so no path can escape workDir.
type SourceService struct {
	serverCfg    config.ServerConfig
	sourcesCfg   config.SourcesConfig
	tableService tableImporter

	mu      sync.Mutex
	sources []Source
}

// NewSourceService builds a SourceService scanning cfg.WorkDir and
// importing through tableService.
func NewSourceService(serverCfg config.ServerConfig, sourcesCfg config.SourcesConfig, tableService tableImporter) *SourceService {
	return &SourceService{serverCfg: serverCfg, sourcesCfg: sourcesCfg, tableService: tableService}
}

// MaxUploadBytes returns the largest file Upload will accept, used by the
// router to size the request body limit on /sources/upload.
func (s *SourceService) MaxUploadBytes() int64 {
	return s.sourcesCfg.MaxSizeBytes
}

// AllowedExtensions returns the file extensions Upload and Refresh accept,
// as configured in sources.allowedExtensions.
func (s *SourceService) AllowedExtensions() []string {
	return slices.Clone(s.sourcesCfg.AllowedExtensions)
}

// Refresh rescans workDir and replaces the in-memory cache. Files beyond
// MaxSizeBytes or with a non-whitelisted extension are silently excluded,
// not reported as an error.
func (s *SourceService) Refresh(ctx context.Context) ([]Source, error) {
	entries, err := os.ReadDir(s.serverCfg.SourcesPath())
	if err != nil {
		return nil, NewDisplayError("unable to scan the sources directory", err)
	}

	sources := []Source{}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := filepath.Ext(entry.Name())
		if !slices.Contains(s.sourcesCfg.AllowedExtensions, ext) {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if info.Size() > s.sourcesCfg.MaxSizeBytes {
			continue
		}

		source := Source{
			Name:      entry.Name(),
			Ext:       ext,
			SizeBytes: info.Size(),
			ModTime:   info.ModTime(),
		}
		if format, ok := lookupSourceFormat(ext); ok && format.MultiLayer {
			source.Layers = s.listLayers(ctx, entry.Name())
		}
		sources = append(sources, source)
	}

	s.mu.Lock()
	s.sources = sources
	s.mu.Unlock()
	return sources, nil
}

// listLayers enumerates a multi-layer source's layers. An unreadable file
// yields no layer rather than failing the whole Refresh.
func (s *SourceService) listLayers(ctx context.Context, name string) []string {
	layers, err := s.tableService.ListLayers(ctx, filepath.Join(s.serverCfg.SourcesPath(), name))
	if err != nil {
		return nil
	}
	return layers
}

// sourceSortFields is the allowlist of sortable fields, mapped to their
// comparator — Source has no query to validate an arbitrary column against.
var sourceSortFields = map[string]func(a, b Source) int{
	"name":      func(a, b Source) int { return cmp.Compare(a.Name, b.Name) },
	"sizeBytes": func(a, b Source) int { return cmp.Compare(a.SizeBytes, b.SizeBytes) },
	"modTime":   func(a, b Source) int { return a.ModTime.Compare(b.ModTime) },
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

// List returns one page of the cache from the last Refresh, ordered by sort
// or by scan order when empty, plus the total count. It never rescans the
// disk, and sorts a copy so a caller cannot mutate the cache.
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
			out[i].Layers = append([]string(nil), src.Layers...)
		}
	}
	return out
}

// Upload writes content to workDir under filename, sanitized to its base name
// and validated against the same extension whitelist and size limit as
// Refresh. An existing file is a conflict, never silently overwritten.
func (s *SourceService) Upload(filename string, content io.Reader) (string, error) {
	filename = filepath.Base(filename)
	ext := filepath.Ext(filename)
	if !slices.Contains(s.sourcesCfg.AllowedExtensions, ext) {
		return "", fmt.Errorf("extension not allowed: %q", ext)
	}

	path := filepath.Join(s.serverCfg.SourcesPath(), filename)
	out, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		if os.IsExist(err) {
			return "", &ConflictError{fmt.Sprintf("a file named %q already exists in workDir", filename)}
		}
		return "", NewDisplayError(fmt.Sprintf("unable to create file %q in workDir", filename), err)
	}
	defer out.Close()

	written, err := io.Copy(out, io.LimitReader(content, s.sourcesCfg.MaxSizeBytes+1))
	if err != nil {
		os.Remove(path)
		return "", NewDisplayError(fmt.Sprintf("unable to write file %q", filename), err)
	}
	if written > s.sourcesCfg.MaxSizeBytes {
		os.Remove(path)
		return "", fmt.Errorf("file exceeds maxSizeBytes (%d bytes)", s.sourcesCfg.MaxSizeBytes)
	}
	return filename, nil
}

// Load imports the cached source into a new table and returns its tableId. A
// multi-layer source is loaded one named layer at a time; a zero srid detects
// the CRS from the file. The same source may be loaded any number of times.
func (s *SourceService) Load(ctx context.Context, name, layer, tableName string, srid int) (string, error) {
	found, loadedName, err := s.resolveLoad(name, layer)
	if err != nil {
		return "", err
	}
	if tableName == "" {
		tableName = loadedName
	}
	return s.tableService.ImportFromFile(ctx, loadedName, tableName, s.pathOf(found.Name), found.Ext, layer, srid)
}

// Reload replaces tableId's data with a fresh import of the cached source,
// keeping the table itself — its name, its settings and its filters. The
// source is resolved exactly as Load resolves it.
func (s *SourceService) Reload(ctx context.Context, tableId, name, layer string, srid int) error {
	found, loadedName, err := s.resolveLoad(name, layer)
	if err != nil {
		return err
	}
	return s.tableService.ReloadFromFile(ctx, tableId, loadedName, s.pathOf(found.Name), layer, srid)
}

// resolveLoad validates name and layer against the cache and returns the entry
// plus the composite name a load registers the table under.
func (s *SourceService) resolveLoad(name, layer string) (Source, string, error) {
	found, err := s.findSource(name)
	if err != nil {
		return Source{}, "", err
	}

	format, known := lookupSourceFormat(found.Ext)
	if !known || !format.MultiLayer {
		if layer != "" {
			return Source{}, "", &ValidationError{fmt.Sprintf("source %q does not support layer selection", name)}
		}
	} else {
		if layer == "" {
			return Source{}, "", &ValidationError{fmt.Sprintf("source %q has multiple layers — specify one", name)}
		}
		if !slices.Contains(found.Layers, layer) {
			return Source{}, "", &NotFoundError{fmt.Sprintf("unknown layer %q for source %q — call /sources/refresh first", layer, name)}
		}
	}

	loadedName := found.Name
	if layer != "" {
		loadedName += layerSeparator + layer
	}
	return found, loadedName, nil
}

// findSource returns a copy of the cached entry named name, or a NotFoundError
// — the only way a name from a request is turned into a file.
func (s *SourceService) findSource(name string) (Source, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, source := range s.sources {
		if source.Name == name {
			return source, nil
		}
	}
	return Source{}, &NotFoundError{fmt.Sprintf("unknown source %q — call /sources/refresh first", name)}
}

func (s *SourceService) pathOf(name string) string {
	return filepath.Join(s.serverCfg.SourcesPath(), name)
}

// sourcePath resolves name, and for a multi-layer source layer, against the
// cache: a name is never turned into a path without this check.
func (s *SourceService) sourcePath(name, layer string) (string, error) {
	found, err := s.findSource(name)
	if err != nil {
		return "", err
	}

	if format, known := lookupSourceFormat(found.Ext); known && format.MultiLayer && layer != "" {
		if !slices.Contains(found.Layers, layer) {
			return "", &NotFoundError{fmt.Sprintf("unknown layer %q for source %q — call /sources/refresh first", layer, name)}
		}
	}

	return s.pathOf(found.Name), nil
}

// DetectCRS reports the authority/code of a source's declared CRS, 0 if
// undetectable, so a caller can preview it before Load. A format that is not
// Introspectable reports no CRS rather than calling into storage.
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

// SourceHasGeometry reports whether a source carries a geometry field, which
// is what tells a caller whether the CRS question applies to it. Every format
// is asked, each read the way CreateFromFile reads it.
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

// Delete erases the source file from workDir and drops its cache entry. The
// tables loaded from it are left alone: they hold their own copy of the data
// and are independent of the file they came from.
func (s *SourceService) Delete(name string) error {
	found, err := s.findSource(name)
	if err != nil {
		return err
	}

	if err := os.Remove(s.pathOf(found.Name)); err != nil && !os.IsNotExist(err) {
		return NewDisplayError(fmt.Sprintf("unable to delete file %q from workDir", name), err)
	}

	s.mu.Lock()
	s.sources = slices.DeleteFunc(s.sources, func(source Source) bool { return source.Name == name })
	s.mu.Unlock()
	return nil
}
