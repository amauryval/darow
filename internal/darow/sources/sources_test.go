package sources

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/MyAppLibs/go-pkg/apperr"
)

func setupFiles(t *testing.T) *Files {
	t.Helper()

	return New(filepath.Join(t.TempDir(), "sources"), 10)
}

func Test_Files_Save(t *testing.T) {
	cases := []struct {
		name    string
		file    string
		content string
		wantErr error
	}{
		{name: "Saves a source", file: "a.geojson", content: "{}"},
		{name: "Saves a source whose extension is in capitals", file: "a.CSV", content: "x"},
		{name: "Refuses a path", file: "../a.csv", content: "x", wantErr: apperr.ErrInvalid},
		{name: "Refuses a hidden file", file: ".a.csv", content: "x", wantErr: apperr.ErrInvalid},
		{name: "Refuses another kind of file", file: "a.exe", content: "x", wantErr: apperr.ErrInvalid},
		{name: "Refuses a file over the limit", file: "a.csv", content: strings.Repeat("x", 11), wantErr: apperr.ErrInvalid},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			files := setupFiles(t)

			source, err := files.Save(c.file, strings.NewReader(c.content))

			if c.wantErr != nil {
				assert.ErrorIs(t, err, c.wantErr)
				list, err := files.List()
				require.NoError(t, err)
				assert.Empty(t, list)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, int64(len(c.content)), source.Size)
		})
	}
}

func Test_Files_List(t *testing.T) {
	files := setupFiles(t)
	_, err := files.Save("b.csv", strings.NewReader("b"))
	require.NoError(t, err)
	_, err = files.Save("a.csv", strings.NewReader("a"))
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(files.dir, "notes.txt"), []byte("x"), 0o600))

	list, err := files.List()

	require.NoError(t, err)
	require.Len(t, list, 2)
	assert.Equal(t, "a.csv", list[0].Name)
	assert.Equal(t, "b.csv", list[1].Name)
}

func Test_Files_Path(t *testing.T) {
	files := setupFiles(t)
	_, err := files.Save("a.csv", strings.NewReader("a"))
	require.NoError(t, err)

	path, err := files.Path("a.csv")
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(files.dir, "a.csv"), path)
	_, err = files.Path("b.csv")
	assert.ErrorIs(t, err, apperr.ErrNotFound)
}

func Test_Files_Delete(t *testing.T) {
	files := setupFiles(t)
	_, err := files.Save("a.csv", strings.NewReader("a"))
	require.NoError(t, err)

	require.NoError(t, files.Delete("a.csv"))

	assert.ErrorIs(t, files.Delete("a.csv"), apperr.ErrNotFound)
}

func Test_Files_MaxBytes(t *testing.T) {
	assert.Equal(t, int64(10), setupFiles(t).MaxBytes())
}
