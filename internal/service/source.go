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

// Source describes a file found in workDir, whether or not it has been
// loaded into a table yet.
type Source struct {
	Name      string    `json:"name"`
	Ext       string    `json:"ext"`
	SizeBytes int64     `json:"sizeBytes"`
	ModTime   time.Time `json:"modTime"`
	Loaded    bool      `json:"loaded"`
	TableId   string    `json:"tableId,omitempty"`
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

		tableId, isLoaded := loaded[entry.Name()]
		sources = append(sources, Source{
			Name:      entry.Name(),
			Ext:       ext,
			SizeBytes: info.Size(),
			ModTime:   info.ModTime(),
			Loaded:    isLoaded,
			TableId:   tableId,
		})
	}

	s.mu.Lock()
	s.sources = sources
	s.mu.Unlock()
	return sources, nil
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
// current cache (règle 15). Returns the newly created tableId.
func (s *SourceService) Load(name string) (string, error) {
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
	if found.Loaded {
		return "", fmt.Errorf("source %q already loaded (table %s)", name, found.TableId)
	}

	path := filepath.Join(s.cfg.WorkDir, found.Name)
	return s.tableService.ImportFromFile(found.Name, path, found.Ext)
}
