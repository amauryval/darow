package tables

import (
	"darow/internal/storage"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_Resolve_ScalarTable(t *testing.T) {
	table, err := resolve(storage.Metadata{PhysicalName: "t_abc", IsGeo: false})
	require.NoError(t, err)

	assert.Equal(t, "t_abc", table.PhysicalName)
	assert.Empty(t, table.QuotedGeometryColumn)
}

func Test_Resolve_GeoTable(t *testing.T) {
	table, err := resolve(storage.Metadata{PhysicalName: "t_abc", IsGeo: true, GeometryColumn: "geom"})
	require.NoError(t, err)

	assert.Equal(t, "t_abc", table.PhysicalName)
	assert.Equal(t, `"geom"`, table.QuotedGeometryColumn)
}

func Test_Resolve_InvalidPhysicalName(t *testing.T) {
	_, err := resolve(storage.Metadata{PhysicalName: "not valid; DROP TABLE x"})
	assert.Error(t, err)
}
