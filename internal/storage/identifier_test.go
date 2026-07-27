package storage

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_ValidateIdentifier(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{name: "simple lowercase", input: "buildings"},
		{name: "with underscore prefix", input: "_pkey"},
		{name: "with digits", input: "t_a1b2c3"},
		{name: "empty string", input: "", wantErr: true},
		{name: "starts with digit", input: "1table", wantErr: true},
		{name: "contains space", input: "my table", wantErr: true},
		{name: "contains semicolon", input: "table;DROP TABLE x", wantErr: true},
		{name: "contains dash", input: "my-table", wantErr: true},
		{name: "contains dot", input: "schema.table", wantErr: true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := ValidateIdentifier(c.input)

			if c.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
		})
	}
}
