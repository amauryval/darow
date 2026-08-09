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

func Test_WrittenGeometryValues(t *testing.T) {
	rows := []map[string]any{
		{"geom": storage.GeometryValue{Literal: "POINT(1 1)", Format: storage.GeometryWKT}},
		{"geom": "not a geometry value"},
		{"geom": storage.GeometryValue{Literal: "", Format: storage.GeometryWKT}},
		{"other": "irrelevant"},
	}

	values := writtenGeometryValues(rows, "geom")

	assert.Equal(t, []storage.GeometryValue{{Literal: "POINT(1 1)", Format: storage.GeometryWKT}}, values)
}

func Test_RowService_CheckGeometryValidity(t *testing.T) {
	cases := []struct {
		name      string
		written   []any
		results   []bool
		failCheck bool
		wantErr   bool
	}{
		{
			name:    "a valid geometry is accepted",
			written: []any{storage.GeometryValue{Literal: "POINT(1 1)", Format: storage.GeometryWKT}},
			results: []bool{true},
		},
		{
			name:    "a self-intersecting polygon is refused",
			written: []any{storage.GeometryValue{Literal: "POLYGON((0 0, 2 2, 2 0, 0 2, 0 0))", Format: storage.GeometryWKT}},
			results: []bool{false},
			wantErr: true,
		},
		{name: "nothing written skips the check", written: []any{"not a geometry value"}},
		{
			name:      "a failed check is reported",
			written:   []any{storage.GeometryValue{Literal: "POINT(1 1)", Format: storage.GeometryWKT}},
			failCheck: true,
			wantErr:   true,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			svc, _, _, tableRepo := setupRowService(t)
			meta := storage.Metadata{TableId: "id-1", IsGeo: true, GeometryColumn: "geom"}

			if c.results != nil || c.failCheck {
				geoRepo := &mocks.GeoRepository{}
				tableRepo.On("Geo").Return(geoRepo, true)
				if c.failCheck {
					geoRepo.On("ValidateGeometries", mock.Anything, mock.Anything).Return(nil, errors.New("duckdb: connection reset")).Once()
				} else {
					geoRepo.On("ValidateGeometries", mock.Anything, mock.Anything).Return(c.results, nil).Once()
				}
			}

			rows := make([]map[string]any, len(c.written))
			for i, value := range c.written {
				rows[i] = map[string]any{"geom": value}
			}

			err := svc.checkGeometryValidity(context.Background(), meta, rows)

			if !c.wantErr {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			if !c.failCheck {
				var validationErr *ValidationError
				require.ErrorAs(t, err, &validationErr)
				assert.Contains(t, validationErr.Error(), c.written[0].(storage.GeometryValue).Literal)
			}
		})
	}
}

// A table with no geometry column never runs the check.
func Test_RowService_CheckGeometryValidity_SkipsScalarTable(t *testing.T) {
	svc, _, _, _ := setupRowService(t)

	err := svc.checkGeometryValidity(context.Background(), storage.Metadata{TableId: "id-1"}, []map[string]any{
		{"geom": storage.GeometryValue{Literal: "POINT(1 1)", Format: storage.GeometryWKT}},
	})

	require.NoError(t, err)
}

// The refusal happens before anything is written: an invalid geometry never
// reaches the repository, and the family check — which passes here — never
// masks it.
func Test_RowService_InsertRow_RefusesAnInvalidGeometry(t *testing.T) {
	svc, _, _, tableRepo := setupRowService(t)
	geoRepo := &mocks.GeoRepository{}
	tableRepo.On("Geo").Return(geoRepo, true)
	geoRepo.On("Types", mock.Anything).Return([]string{}, nil).Once()
	geoRepo.On("ValidateGeometries", mock.Anything, mock.Anything).Return([]bool{false}, nil).Once()

	_, err := svc.InsertRow(
		context.Background(),
		storage.Metadata{TableId: "id-1", IsGeo: true, GeometryColumn: "geom"},
		map[string]any{"geom": "POLYGON((0 0, 2 2, 2 0, 0 2, 0 0))"},
		storage.DefaultGeometryCRS,
	)

	var validationErr *ValidationError
	require.ErrorAs(t, err, &validationErr)
	assert.Contains(t, validationErr.Error(), "POLYGON((0 0, 2 2, 2 0, 0 2, 0 0))")
}
