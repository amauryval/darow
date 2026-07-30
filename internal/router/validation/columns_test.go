package validation

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"gogeoserver/internal/service"
	"gogeoserver/internal/storage/catalog"
)

func Test_NewColumnValidator(t *testing.T) {
	schemaSvc := &mockSchemaGetter{}

	cv := NewColumnValidator(schemaSvc)

	assert.Same(t, schemaSvc, cv.schemaService)
}

func Test_ColumnValidator_ValidateColumns(t *testing.T) {
	cases := []struct {
		name      string
		requested []string
		schemaErr error
		wantErr   bool
	}{
		{name: "no columns requested skips validation", requested: nil},
		{name: "valid column passes", requested: []string{"status"}},
		{name: "geometry column is not a valid target", requested: []string{"geom"}, wantErr: true},
		{name: "unknown column is rejected", requested: []string{"bogus"}, wantErr: true},
		{name: "schema failure propagates", requested: []string{"status"}, schemaErr: assert.AnError, wantErr: true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			schemaSvc := &mockSchemaGetter{}
			cv := ColumnValidator{schemaService: schemaSvc}

			meta := catalog.Metadata{TableId: "t1", GeometryColumn: "geom"}
			if len(c.requested) > 0 {
				schemaSvc.On("Schema", mock.Anything, meta).
					Return([]service.ColumnSchema{{Name: "status", Type: "VARCHAR"}, {Name: "geom", Type: "GEOMETRY"}}, c.schemaErr).Once()
			}

			err := cv.ValidateColumns(context.Background(), meta, c.requested)

			if c.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			schemaSvc.AssertExpectations(t)
		})
	}
}
