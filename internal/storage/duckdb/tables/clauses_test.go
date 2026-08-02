package tables

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"darow/internal/storage"
)

func Test_placeholders(t *testing.T) {
	assert.Equal(t, "", placeholders(0))
	assert.Equal(t, "?", placeholders(1))
	assert.Equal(t, "?, ?, ?", placeholders(3))
}

func Test_pkeysToArgs(t *testing.T) {
	args := pkeysToArgs([]string{"a", "b"})
	assert.Equal(t, []any{"a", "b"}, args)
}

func Test_PkeyInClause(t *testing.T) {
	clause, args := pkeyInClause([]string{"a", "b"})
	assert.Equal(t, "__pkey IN (?, ?)", clause)
	assert.Equal(t, []any{"a", "b"}, args)
}

// An empty pkeys must never produce "__pkey IN ()", a SQL syntax error in
// DuckDB — every pkey-scoped caller relies on this guard instead of
// duplicating it.
func Test_PkeyInClause_EmptyPkeysIsSafeNoOp(t *testing.T) {
	clause, args := pkeyInClause(nil)
	assert.Equal(t, "FALSE", clause)
	assert.Empty(t, args)
}

func Test_AttrProjection_NoAttrs_ExcludesGeometryWhenGeo(t *testing.T) {
	geoTable, err := resolve(storage.Metadata{PhysicalName: "t_a", IsGeo: true, GeometryColumn: "geom"})
	require.NoError(t, err)

	projection, err := attrProjection(geoTable, nil)
	require.NoError(t, err)
	assert.Equal(t, `* EXCLUDE ("geom")`, projection)
}

func Test_AttrProjection_NoAttrs_ScalarTable(t *testing.T) {
	scalarTable, err := resolve(storage.Metadata{PhysicalName: "t_a"})
	require.NoError(t, err)

	projection, err := attrProjection(scalarTable, nil)
	require.NoError(t, err)
	assert.Equal(t, "*", projection)
}

func Test_AttrProjection_WithAttrs_QuotesEach(t *testing.T) {
	geoTable, err := resolve(storage.Metadata{PhysicalName: "t_a", IsGeo: true, GeometryColumn: "geom"})
	require.NoError(t, err)

	projection, err := attrProjection(geoTable, []string{"name", "height"})
	require.NoError(t, err)
	assert.Equal(t, `"name", "height"`, projection)
}

func Test_AttrProjection_RejectsInvalidAttr(t *testing.T) {
	scalarTable, err := resolve(storage.Metadata{PhysicalName: "t_a"})
	require.NoError(t, err)

	_, err = attrProjection(scalarTable, []string{""})
	assert.Error(t, err)
}

// featureAttrProjection is used by queries that already select __pkey
// explicitly (Features, FeaturesInTile) — an empty attrs must exclude it
// from the "every column" case too, or __pkey ends up selected twice.
func Test_FeatureAttrProjection_NoAttrs_ExcludesPkeyAndGeometry(t *testing.T) {
	geoTable, err := resolve(storage.Metadata{PhysicalName: "t_a", IsGeo: true, GeometryColumn: "geom"})
	require.NoError(t, err)

	projection, err := featureAttrProjection(geoTable, nil)
	require.NoError(t, err)
	assert.Equal(t, `* EXCLUDE (__pkey, "geom")`, projection)
}

func Test_FeatureAttrProjection_WithAttrs_SameAsAttrProjection(t *testing.T) {
	geoTable, err := resolve(storage.Metadata{PhysicalName: "t_a", IsGeo: true, GeometryColumn: "geom"})
	require.NoError(t, err)

	projection, err := featureAttrProjection(geoTable, []string{"name"})
	require.NoError(t, err)
	assert.Equal(t, `"name"`, projection)
}

func Test_OrderBy_EmptySort_OrdersByPkeyOnly(t *testing.T) {
	orderBy, err := orderByClause(nil)
	require.NoError(t, err)
	assert.Equal(t, "__pkey", orderBy)
}

func Test_OrderBy_QuotesColumnsAndAppendsPkeyTiebreaker(t *testing.T) {
	orderBy, err := orderByClause([]storage.SortField{{Column: "name"}, {Column: "height", Desc: true}})
	require.NoError(t, err)
	assert.Equal(t, `"name", "height" DESC, __pkey`, orderBy)
}

func Test_OrderBy_RejectsInvalidColumn(t *testing.T) {
	_, err := orderByClause([]storage.SortField{{Column: ""}})
	assert.Error(t, err)
}
