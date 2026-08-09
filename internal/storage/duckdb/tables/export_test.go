package tables

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"darow/internal/storage"
)

func Test_table_CopyTo(t *testing.T) {
	cases := []struct {
		name        string
		format      storage.ExportFormat
		targetCRS   string
		wantContent []string
		wantAbsent  []string
	}{
		{
			name:        "CSV carries the columns and no geometry",
			format:      storage.ExportCSV,
			targetCRS:   "OGC:CRS84",
			wantContent: []string{"pkey,name", "a", "b"},
			wantAbsent:  []string{"POINT"},
		},
		{
			name:        "GeoJSON is a FeatureCollection in lon/lat",
			format:      storage.ExportGeoJSON,
			targetCRS:   "OGC:CRS84",
			wantContent: []string{`"type": "FeatureCollection"`, `"name": "points"`, `"type": "Feature"`, `"name": "a"`, "0.898"},
		},
		{
			name:        "GeoJSON reprojects to the requested CRS",
			format:      storage.ExportGeoJSON,
			targetCRS:   "EPSG:2154",
			wantContent: []string{`"type": "FeatureCollection"`, "390697"},
			wantAbsent:  []string{"0.8983"},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			conn := openRepoTestDB(t)
			repo, err := Open(conn, conn, newGeoTestTable(t, conn))
			require.NoError(t, err)

			dest := filepath.Join(t.TempDir(), "export.out")
			require.NoError(t, repo.CopyTo(context.Background(), storage.ExportRequest{
				DestPath:  dest,
				Format:    c.format,
				TargetCRS: c.targetCRS,
				LayerName: "points",
			}))

			written, err := os.ReadFile(dest)
			require.NoError(t, err)
			for _, want := range c.wantContent {
				assert.Contains(t, string(written), want)
			}
			for _, absent := range c.wantAbsent {
				assert.NotContains(t, string(written), absent)
			}
		})
	}
}

func Test_table_CopyTo_Parquet(t *testing.T) {
	cases := []struct {
		name      string
		targetCRS string
		wantX     float64
		wantY     float64
		delta     float64
	}{
		{name: "CRS84 writes lon/lat", targetCRS: "OGC:CRS84", wantX: 0.898, wantY: 0.898, delta: 0.01},
		{name: "an arbitrary CRS reprojects", targetCRS: "EPSG:2154", wantX: 390697, wantY: 1037552, delta: 1},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			conn := openRepoTestDB(t)
			repo, err := Open(conn, conn, newGeoTestTable(t, conn))
			require.NoError(t, err)

			dest := filepath.Join(t.TempDir(), "export.parquet")
			require.NoError(t, repo.CopyTo(context.Background(), storage.ExportRequest{
				DestPath: dest, Format: storage.ExportParquet, TargetCRS: c.targetCRS,
			}))

			var name, wkt string
			require.NoError(t, conn.db.QueryRow(
				`SELECT name, ST_AsText(geometry) FROM read_parquet(?) WHERE pkey = '1'`, dest,
			).Scan(&name, &wkt))

			assert.Equal(t, "a", name)
			assertPointNear(t, wkt, c.wantX, c.wantY, c.delta)
		})
	}
}

// The GeoParquet metadata DuckDB attaches carries no `crs` key, whatever the
// geometry was reprojected to — its GEOMETRY type holds no SRID, so the
// writer cannot know. A reader applies the format's default, OGC:CRS84, so
// an export in any other CRS is only identified by its file name.
func Test_table_CopyTo_ParquetGeoMetadataHasNoCRS(t *testing.T) {
	conn := openRepoTestDB(t)
	repo, err := Open(conn, conn, newGeoTestTable(t, conn))
	require.NoError(t, err)

	dest := filepath.Join(t.TempDir(), "export.parquet")
	require.NoError(t, repo.CopyTo(context.Background(), storage.ExportRequest{
		DestPath: dest, Format: storage.ExportParquet, TargetCRS: "EPSG:2154",
	}))

	var metadata string
	require.NoError(t, conn.db.QueryRow(
		`SELECT value FROM parquet_kv_metadata(?) WHERE key = 'geo'`, dest,
	).Scan(&metadata))

	assert.Contains(t, metadata, `"primary_column":"geometry"`)
	assert.NotContains(t, metadata, `"crs"`)
}

func Test_table_CopyTo_RejectsUnknownCRS(t *testing.T) {
	conn := openRepoTestDB(t)
	repo, err := Open(conn, conn, newGeoTestTable(t, conn))
	require.NoError(t, err)

	dest := filepath.Join(t.TempDir(), "export.parquet")
	err = repo.CopyTo(context.Background(), storage.ExportRequest{
		DestPath: dest, Format: storage.ExportParquet, TargetCRS: "EPSG:999999",
	})

	var validation *storage.ValidationError
	require.ErrorAs(t, err, &validation)
	assert.NoFileExists(t, dest, "a refused CRS writes nothing")
}

func Test_table_CopyTo_RejectsUnknownFormat(t *testing.T) {
	conn := openRepoTestDB(t)
	repo, err := Open(conn, conn, newGeoTestTable(t, conn))
	require.NoError(t, err)

	err = repo.CopyTo(context.Background(), storage.ExportRequest{
		DestPath: filepath.Join(t.TempDir(), "export.out"), Format: "shapefile",
	})

	var validation *storage.ValidationError
	require.ErrorAs(t, err, &validation)
}

func Test_table_CopyTo_NonGeoTable(t *testing.T) {
	conn := openRepoTestDB(t)
	repo, err := Open(conn, conn, newScalarTestTable(t, conn))
	require.NoError(t, err)

	dest := filepath.Join(t.TempDir(), "export.parquet")
	require.NoError(t, repo.CopyTo(context.Background(), storage.ExportRequest{
		DestPath: dest, Format: storage.ExportParquet, TargetCRS: "OGC:CRS84",
	}))

	var count int64
	require.NoError(t, conn.db.QueryRow(`SELECT COUNT(*) FROM read_parquet(?)`, dest).Scan(&count))
	assert.Positive(t, count)
}
