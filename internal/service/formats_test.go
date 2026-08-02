package service

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_LookupSourceFormat(t *testing.T) {
	format, ok := lookupSourceFormat(".gpkg")
	assert.True(t, ok)
	assert.True(t, format.MultiLayer)
	assert.True(t, format.Introspectable)

	format, ok = lookupSourceFormat(".geojson")
	assert.True(t, ok)
	assert.False(t, format.MultiLayer)
	assert.True(t, format.Introspectable)

	format, ok = lookupSourceFormat(".parquet")
	assert.True(t, ok)
	assert.False(t, format.MultiLayer)
	assert.False(t, format.Introspectable)

	_, ok = lookupSourceFormat(".unknown")
	assert.False(t, ok)
}
