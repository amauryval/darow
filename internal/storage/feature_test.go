package storage

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_GeoFeature_ZeroValue(t *testing.T) {
	var f GeoFeature

	assert.Empty(t, f.Pkey)
	assert.Empty(t, f.GeoJSON)
	assert.Nil(t, f.Values)
}
