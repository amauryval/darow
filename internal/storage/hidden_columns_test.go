package storage

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_IsHiddenColumn(t *testing.T) {
	cases := []struct {
		name string
		col  string
		want bool
	}{
		{name: "pkey is hidden", col: "__pkey", want: true},
		{name: "ogc_fid is hidden", col: "OGC_FID", want: true},
		{name: "a regular column is not hidden", col: "height", want: false},
		{name: "lowercase ogc_fid is not the reserved name", col: "ogc_fid", want: false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, IsHiddenColumn(c.col))
		})
	}
}
