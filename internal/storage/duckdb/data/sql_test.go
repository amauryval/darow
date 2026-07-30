package data

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gogeoserver/internal/storage"
	"gogeoserver/internal/storage/catalog"
)

func Test_Placeholders(t *testing.T) {
	assert.Equal(t, "", Placeholders(0))
	assert.Equal(t, "?", Placeholders(1))
	assert.Equal(t, "?, ?, ?", Placeholders(3))
}

func Test_PkeysToArgs(t *testing.T) {
	args := PkeysToArgs([]string{"a", "b"})
	assert.Equal(t, []any{"a", "b"}, args)
}

func Test_PkeyInClause(t *testing.T) {
	clause, args := PkeyInClause([]string{"a", "b"})
	assert.Equal(t, "__pkey IN (?, ?)", clause)
	assert.Equal(t, []any{"a", "b"}, args)
}

func Test_AttrProjection_NoAttrs_ExcludesGeometryWhenGeo(t *testing.T) {
	geoTable, err := Resolve(catalog.Metadata{PhysicalName: "t_a", IsGeo: true, GeometryColumn: "geom"})
	require.NoError(t, err)

	projection, err := AttrProjection(geoTable, nil)
	require.NoError(t, err)
	assert.Equal(t, `* EXCLUDE ("geom")`, projection)
}

func Test_AttrProjection_NoAttrs_ScalarTable(t *testing.T) {
	scalarTable, err := Resolve(catalog.Metadata{PhysicalName: "t_a"})
	require.NoError(t, err)

	projection, err := AttrProjection(scalarTable, nil)
	require.NoError(t, err)
	assert.Equal(t, "*", projection)
}

func Test_AttrProjection_WithAttrs_QuotesEach(t *testing.T) {
	geoTable, err := Resolve(catalog.Metadata{PhysicalName: "t_a", IsGeo: true, GeometryColumn: "geom"})
	require.NoError(t, err)

	projection, err := AttrProjection(geoTable, []string{"name", "height"})
	require.NoError(t, err)
	assert.Equal(t, `"name", "height"`, projection)
}

func Test_AttrProjection_RejectsInvalidAttr(t *testing.T) {
	scalarTable, err := Resolve(catalog.Metadata{PhysicalName: "t_a"})
	require.NoError(t, err)

	_, err = AttrProjection(scalarTable, []string{""})
	assert.Error(t, err)
}

func Test_OrderBy_EmptySort_OrdersByPkeyOnly(t *testing.T) {
	orderBy, err := OrderBy(nil)
	require.NoError(t, err)
	assert.Equal(t, "__pkey", orderBy)
}

func Test_OrderBy_QuotesColumnsAndAppendsPkeyTiebreaker(t *testing.T) {
	orderBy, err := OrderBy([]storage.SortField{{Column: "name"}, {Column: "height", Desc: true}})
	require.NoError(t, err)
	assert.Equal(t, `"name", "height" DESC, __pkey`, orderBy)
}

func Test_OrderBy_RejectsInvalidColumn(t *testing.T) {
	_, err := OrderBy([]storage.SortField{{Column: ""}})
	assert.Error(t, err)
}
