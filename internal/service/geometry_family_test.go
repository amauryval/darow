package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"darow/internal/storage"
	"darow/internal/storage/mocks"
)

func Test_GeometryFamily(t *testing.T) {
	cases := []struct {
		name string
		typ  string
		want string
	}{
		{name: "point", typ: "POINT", want: familyPoint},
		{name: "multipoint", typ: "MULTIPOINT", want: familyPoint},
		{name: "linestring", typ: "LINESTRING", want: familyLine},
		{name: "multilinestring", typ: "MULTILINESTRING", want: familyLine},
		{name: "polygon", typ: "POLYGON", want: familyPolygon},
		{name: "multipolygon", typ: "MULTIPOLYGON", want: familyPolygon},
		{name: "lowercase geojson name", typ: "Polygon", want: familyPolygon},
		{name: "prefixed name", typ: "ST_Point", want: familyPoint},
		{name: "collection belongs to no family", typ: "GEOMETRYCOLLECTION"},
		{name: "unknown name", typ: "TRIANGLE"},
		{name: "empty name"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, geometryFamily(c.typ))
		})
	}
}

func Test_WrittenFamily(t *testing.T) {
	cases := []struct {
		name  string
		value storage.GeometryValue
		want  string
	}{
		{
			name:  "wkt polygon",
			value: storage.GeometryValue{Literal: "POLYGON((0 0,1 0,1 1,0 0))", Format: storage.GeometryWKT},
			want:  familyPolygon,
		},
		{
			name:  "wkt with a space before the coordinates",
			value: storage.GeometryValue{Literal: "MULTILINESTRING ((0 0,1 1))", Format: storage.GeometryWKT},
			want:  familyLine,
		},
		{
			name:  "geojson point",
			value: storage.GeometryValue{Literal: `{"type":"Point","coordinates":[1,2]}`, Format: storage.GeometryGeoJSON},
			want:  familyPoint,
		},
		{
			name:  "unparseable geojson",
			value: storage.GeometryValue{Literal: "{", Format: storage.GeometryGeoJSON},
		},
		{name: "empty literal writes no geometry"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, writtenFamily(c.value))
		})
	}
}

// A table already holding one family refuses another, whatever the encoding
// the geometry was written in; an empty table and a table already mixing
// families take anything.
func Test_RowService_CheckGeometryFamily(t *testing.T) {
	cases := []struct {
		name        string
		storedTypes []string
		written     []any
		failTypes   bool
		wantErr     bool
	}{
		{
			name:        "same family as the table",
			storedTypes: []string{"POLYGON", "MULTIPOLYGON"},
			written:     []any{storage.GeometryValue{Literal: "POLYGON((0 0,1 0,1 1,0 0))", Format: storage.GeometryWKT}},
		},
		{
			name:        "point written to a table of polygons",
			storedTypes: []string{"POLYGON"},
			written:     []any{storage.GeometryValue{Literal: `{"type":"Point"}`, Format: storage.GeometryGeoJSON}},
			wantErr:     true,
		},
		{
			name:        "line written to a table of points",
			storedTypes: []string{"MULTIPOINT"},
			written:     []any{storage.GeometryValue{Literal: "LINESTRING(0 0,1 1)", Format: storage.GeometryWKT}},
			wantErr:     true,
		},
		{
			name:        "empty table takes any family",
			storedTypes: []string{},
			written:     []any{storage.GeometryValue{Literal: "POINT(1 1)", Format: storage.GeometryWKT}},
		},
		{
			name:        "already mixed table takes any family",
			storedTypes: []string{"POINT", "POLYGON"},
			written:     []any{storage.GeometryValue{Literal: "LINESTRING(0 0,1 1)", Format: storage.GeometryWKT}},
		},
		{
			name:        "a batch mixing families is refused as a whole",
			storedTypes: []string{"POLYGON"},
			written: []any{
				storage.GeometryValue{Literal: "POLYGON((0 0,1 0,1 1,0 0))", Format: storage.GeometryWKT},
				storage.GeometryValue{Literal: "POINT(1 1)", Format: storage.GeometryWKT},
			},
			wantErr: true,
		},
		{
			name:    "a collection is never refused",
			written: []any{storage.GeometryValue{Literal: `{"type":"GeometryCollection"}`, Format: storage.GeometryGeoJSON}},
		},
		{name: "nothing written reads no type at all", written: []any{"not a geometry value"}},
		{
			name:        "a failed read is reported",
			storedTypes: []string{"POLYGON"},
			written:     []any{storage.GeometryValue{Literal: "POINT(1 1)", Format: storage.GeometryWKT}},
			failTypes:   true,
			wantErr:     true,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			svc, _, _, tableRepo := setupRowService(t)
			meta := storage.Metadata{TableId: "id-1", IsGeo: true, GeometryColumn: "geom"}

			if c.storedTypes != nil {
				geoRepo := &mocks.GeoRepository{}
				tableRepo.On("Geo").Return(geoRepo, true)
				if c.failTypes {
					geoRepo.On("Types", mock.Anything).Return(nil, errors.New("duckdb: connection reset")).Once()
				} else {
					geoRepo.On("Types", mock.Anything).Return(c.storedTypes, nil).Once()
				}
			}

			rows := make([]map[string]any, len(c.written))
			for i, value := range c.written {
				rows[i] = map[string]any{"geom": value}
			}

			err := svc.checkGeometryFamily(context.Background(), meta, rows)

			if !c.wantErr {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			if !c.failTypes {
				var validationErr *ValidationError
				require.ErrorAs(t, err, &validationErr)
				assert.Contains(t, validationErr.Error(), "id-1")
			}
		})
	}
}

// A table with no geometry column never reads its types.
func Test_RowService_CheckGeometryFamily_SkipsScalarTable(t *testing.T) {
	svc, _, _, _ := setupRowService(t)

	err := svc.checkGeometryFamily(context.Background(), storage.Metadata{TableId: "id-1"}, []map[string]any{
		{"geom": storage.GeometryValue{Literal: "POINT(1 1)", Format: storage.GeometryWKT}},
	})

	require.NoError(t, err)
}

// The refusal happens before anything is written: a contradicting geometry
// never reaches the repository.
func Test_RowService_InsertRow_RefusesAForeignFamily(t *testing.T) {
	svc, _, _, tableRepo := setupRowService(t)
	geoRepo := &mocks.GeoRepository{}
	tableRepo.On("Geo").Return(geoRepo, true)
	geoRepo.On("Types", mock.Anything).Return([]string{"POLYGON"}, nil).Once()

	_, err := svc.InsertRow(
		context.Background(),
		storage.Metadata{TableId: "id-1", IsGeo: true, GeometryColumn: "geom"},
		map[string]any{"geom": "POINT(1 1)"},
		storage.DefaultGeometryCRS,
	)

	var validationErr *ValidationError
	require.ErrorAs(t, err, &validationErr)
}
