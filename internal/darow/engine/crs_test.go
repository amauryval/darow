package engine

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_CRSOf(t *testing.T) {
	cases := []struct {
		name string
		srid int
		want string
	}{
		{name: "Reads no code as CRS84", srid: 0, want: CRS84},
		{name: "Reads 4326 as CRS84, longitude first", srid: 4326, want: CRS84},
		{name: "Names any other code", srid: 2154, want: "EPSG:2154"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, CRSOf(c.srid))
		})
	}
}
