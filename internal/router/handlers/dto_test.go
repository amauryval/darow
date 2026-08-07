package handlers

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"darow/internal/storage"
)

func Test_ToMetadataDTO_Srid(t *testing.T) {
	t.Run("a table with no known source CRS omits srid", func(t *testing.T) {
		dto := toMetadataDTO(storage.Metadata{TableId: "t1", IsGeo: false, Srid: 0})
		assert.Nil(t, dto.Srid)
	})

	t.Run("a derived geo table has no source CRS either", func(t *testing.T) {
		dto := toMetadataDTO(storage.Metadata{TableId: "t1", IsGeo: true, Srid: 0})
		assert.Nil(t, dto.Srid)
	})

	t.Run("a table imported from a georeferenced source reports its srid", func(t *testing.T) {
		dto := toMetadataDTO(storage.Metadata{TableId: "t1", IsGeo: true, Srid: 2154})
		require.NotNil(t, dto.Srid)
		assert.Equal(t, 2154, *dto.Srid)
	})
}

func Test_RowToJSON(t *testing.T) {
	t.Run("flattens pkey alongside data columns", func(t *testing.T) {
		row := storage.Row{Pkey: "pk-1", Values: map[string]any{"height": 12}}

		out := rowToJSON(row)

		assert.Equal(t, map[string]any{"pkey": "pk-1", "height": 12}, out)
	})

	t.Run("a data column literally named pkey never overwrites the row's real primary key", func(t *testing.T) {
		row := storage.Row{Pkey: "pk-1", Values: map[string]any{"pkey": "not-the-real-one", "height": 12}}

		out := rowToJSON(row)

		assert.Equal(t, map[string]any{"pkey": "pk-1", "height": 12}, out)
	})
}
