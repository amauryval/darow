package storage

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_GeoQuery_ZeroValue(t *testing.T) {
	var q GeoQuery

	assert.Empty(t, q.Filter)
	assert.Nil(t, q.Args)
	assert.Nil(t, q.Tolerance)
}
