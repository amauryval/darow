package service

import (
	"bytes"
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"darow/internal/storage"
	"darow/internal/storage/mocks"
)

// fakeTableResolver hands ExportService the one repository every case works
// against, so a test states what CopyTo does and nothing else.
type fakeTableResolver struct {
	repo storage.TableRepository
	err  error
}

func (f fakeTableResolver) Table(meta storage.Metadata) (storage.TableRepository, error) {
	return f.repo, f.err
}

func setupExportService(t *testing.T, written string, copyErr error) (*ExportService, *mocks.TableRepository) {
	t.Helper()
	repo := &mocks.TableRepository{}
	call := repo.On("CopyTo", mock.Anything, mock.Anything)
	if copyErr != nil {
		call.Return(copyErr)
	} else {
		call.Run(func(args mock.Arguments) {
			req, ok := args.Get(1).(storage.ExportRequest)
			require.True(t, ok)
			require.NoError(t, os.WriteFile(req.DestPath, []byte(written), 0o600))
		}).Return(nil)
	}
	return NewExportService(fakeTableResolver{repo: repo}), repo
}

func Test_ExportService_ContentType(t *testing.T) {
	cases := []struct {
		name        string
		format      storage.ExportFormat
		isGeo       bool
		wantType    string
		wantInvalid bool
	}{
		{name: "csv", format: storage.ExportCSV, wantType: "text/csv"},
		{name: "csv on a geo table", format: storage.ExportCSV, isGeo: true, wantType: "text/csv"},
		{name: "geojson on a geo table", format: storage.ExportGeoJSON, isGeo: true, wantType: "application/geo+json"},
		{name: "geojson on a table without geometry", format: storage.ExportGeoJSON, wantInvalid: true},
		{name: "parquet", format: storage.ExportParquet, wantType: "application/vnd.apache.parquet"},
		{name: "parquet on a geo table", format: storage.ExportParquet, isGeo: true, wantType: "application/vnd.apache.parquet"},
		{name: "unsupported format", format: "shapefile", wantInvalid: true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			svc, _ := setupExportService(t, "", nil)

			contentType, err := svc.ContentType(storage.Metadata{TableId: "t1", IsGeo: c.isGeo}, c.format)

			if c.wantInvalid {
				var validation *ExportValidationError
				require.ErrorAs(t, err, &validation)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, c.wantType, contentType)
		})
	}
}

func Test_ExportCRS(t *testing.T) {
	cases := []struct {
		name      string
		requested int
		tableSrid int
		want      string
	}{
		{name: "no request goes back out in the table's own CRS", tableSrid: 2154, want: "EPSG:2154"},
		{name: "an explicit code wins over the table's", requested: 3857, tableSrid: 2154, want: "EPSG:3857"},
		{name: "4326 is CRS84, never EPSG:4326", requested: 4326, want: "OGC:CRS84"},
		{name: "a table stored in 4326 exports as CRS84", tableSrid: 4326, want: "OGC:CRS84"},
		{name: "a table with no recorded CRS falls back to lon/lat", want: "OGC:CRS84"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			meta := storage.Metadata{Srid: c.tableSrid}
			assert.Equal(t, c.want, ExportCRS(c.requested, meta))
		})
	}
}

func Test_FileName(t *testing.T) {
	cases := []struct {
		name      string
		format    storage.ExportFormat
		isGeo     bool
		targetCRS string
		want      string
	}{
		{name: "csv never carries a CRS", format: storage.ExportCSV, isGeo: true, targetCRS: "EPSG:2154", want: "communes.csv"},
		{name: "lon/lat is named too, the default being the table's own CRS", format: storage.ExportGeoJSON, isGeo: true, targetCRS: "OGC:CRS84", want: "communes_OGCCRS84.geojson"},
		{name: "an explicit CRS is named in the file", format: storage.ExportGeoJSON, isGeo: true, targetCRS: "EPSG:2154", want: "communes_EPSG2154.geojson"},
		{name: "parquet too", format: storage.ExportParquet, isGeo: true, targetCRS: "EPSG:3857", want: "communes_EPSG3857.parquet"},
		{name: "a table without geometry carries no CRS", format: storage.ExportParquet, targetCRS: "EPSG:2154", want: "communes.parquet"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			meta := storage.Metadata{TableName: "communes", IsGeo: c.isGeo}
			assert.Equal(t, c.want, FileName(meta, c.format, c.targetCRS))
		})
	}
}

func Test_ExportService_Export(t *testing.T) {
	svc, repo := setupExportService(t, "pkey,name\n1,a\n", nil)

	var out bytes.Buffer
	meta := storage.Metadata{TableId: "t1", TableName: "communes", IsGeo: true}
	require.NoError(t, svc.Export(context.Background(), &out, meta, storage.ExportCSV, "OGC:CRS84"))

	assert.Equal(t, "pkey,name\n1,a\n", out.String())

	request, ok := repo.Calls[0].Arguments.Get(1).(storage.ExportRequest)
	require.True(t, ok)
	assert.Equal(t, storage.ExportCSV, request.Format)
	assert.Equal(t, "OGC:CRS84", request.TargetCRS)
	assert.Equal(t, "communes", request.LayerName, "the FeatureCollection is named after the table")
	assert.NotEmpty(t, request.DestPath)
}

// A COPY that fails writes nothing to the caller, so the router can still
// answer with a status rather than a truncated download.
func Test_ExportService_Export_LeavesTheResponseUntouchedOnFailure(t *testing.T) {
	svc, _ := setupExportService(t, "", assert.AnError)

	var out bytes.Buffer
	err := svc.Export(context.Background(), &out, storage.Metadata{TableId: "t1"}, storage.ExportParquet, "OGC:CRS84")

	require.Error(t, err)
	assert.Empty(t, out.String())
}

func Test_ExportService_Export_RemovesItsTemporaryDirectory(t *testing.T) {
	repo := &mocks.TableRepository{}
	var dest string
	repo.On("CopyTo", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		req, ok := args.Get(1).(storage.ExportRequest)
		require.True(t, ok)
		dest = req.DestPath
		require.NoError(t, os.WriteFile(req.DestPath, []byte("x"), 0o600))
	}).Return(nil)
	svc := NewExportService(fakeTableResolver{repo: repo})

	var out bytes.Buffer
	require.NoError(t, svc.Export(context.Background(), &out, storage.Metadata{TableId: "t1"}, storage.ExportParquet, "OGC:CRS84"))

	assert.NoFileExists(t, dest)
}
