// Package sources keeps the files tables are loaded from, in one directory of the work directory.
package sources

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/MyAppLibs/go-pkg/apperr"
)

// The codes of the failures of a source, labelled by the darow catalog.
const (
	CodeInvalidSourceName apperr.Code = "darow.invalid_source_name"
	CodeSourceTooLarge    apperr.Code = "darow.source_too_large"
)

// Extensions are the kinds of file a source may be.
var Extensions = []Extension{
	ExtensionGeoJSON, ExtensionJSON, ExtensionCSV, ExtensionGeoPackage, ExtensionParquet, ExtensionFlatGeobuf, ExtensionKML,
}

// Files is the directory of the sources, each at most MaxBytes.
type Files struct {
	dir      string
	maxBytes int64
}

// New returns the sources kept in dir, each at most maxBytes.
func New(dir string, maxBytes int64) *Files {
	return &Files{dir: dir, maxBytes: maxBytes}
}

// MaxBytes returns the largest source accepted.
func (f *Files) MaxBytes() int64 {
	return f.maxBytes
}

// List returns every source, by name.
func (f *Files) List() ([]Source, error) {
	entries, err := os.ReadDir(f.dir)
	if errors.Is(err, fs.ErrNotExist) {
		return []Source{}, nil
	}
	if err != nil {
		return nil, err
	}
	sources := []Source{}
	for _, entry := range entries {
		if entry.IsDir() || checkName(entry.Name()) != nil {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return nil, err
		}
		sources = append(sources, Source{Name: entry.Name(), Size: info.Size(), ModifiedAt: info.ModTime().UTC()})
	}

	return sources, nil
}

// Save writes content as the source called name, replacing one of the same name.
func (f *Files) Save(name string, content io.Reader) (Source, error) {
	if err := checkName(name); err != nil {
		return Source{}, err
	}
	if err := os.MkdirAll(f.dir, 0o700); err != nil {
		return Source{}, err
	}
	part, err := os.CreateTemp(f.dir, ".upload-*")
	if err != nil {
		return Source{}, err
	}
	defer func() { _ = os.Remove(part.Name()) }()

	written, err := io.Copy(part, io.LimitReader(content, f.maxBytes+1))
	if err = errors.Join(err, part.Close()); err != nil {
		return Source{}, err
	}
	if written > f.maxBytes {
		return Source{}, invalidFile(CodeSourceTooLarge, map[string]string{"max": strconv.FormatInt(f.maxBytes, 10)})
	}
	if err := os.Rename(part.Name(), filepath.Join(f.dir, name)); err != nil {
		return Source{}, err
	}

	return Source{Name: name, Size: written, ModifiedAt: time.Now().UTC()}, nil
}

// Path returns the file of the source called name.
func (f *Files) Path(name string) (string, error) {
	if err := checkName(name); err != nil {
		return "", err
	}
	path := filepath.Join(f.dir, name)
	if _, err := os.Stat(path); err != nil {
		return "", fmt.Errorf("source %s: %w", name, apperr.ErrNotFound)
	}

	return path, nil
}

// Delete removes the source called name.
func (f *Files) Delete(name string) error {
	path, err := f.Path(name)
	if err != nil {
		return err
	}

	return os.Remove(path)
}

func checkName(name string) error {
	valid := name != "" && filepath.Base(name) == name && !strings.HasPrefix(name, ".") &&
		slices.Contains(Extensions, Extension(strings.ToLower(filepath.Ext(name))))
	if !valid {
		return invalidFile(CodeInvalidSourceName, nil)
	}

	return nil
}

func invalidFile(code apperr.Code, params map[string]string) error {
	return &apperr.ValidationError{Errors: []apperr.FieldError{{Path: "file", Code: code, Params: params}}}
}
