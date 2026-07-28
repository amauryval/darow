package service

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_LookupSourceFormat(t *testing.T) {
	format, ok := lookupSourceFormat(".gpkg")
	assert.True(t, ok)
	assert.True(t, format.MultiLayer)

	format, ok = lookupSourceFormat(".geojson")
	assert.True(t, ok)
	assert.False(t, format.MultiLayer)

	_, ok = lookupSourceFormat(".unknown")
	assert.False(t, ok)
}
