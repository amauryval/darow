package duckdb

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_Adapter_CreateFromQuery_RejectsUnregisteredTable(t *testing.T) {
	a := setupAdapter(t)

	_, _, err := a.CreateFromQuery(context.Background(), "t_derived", "SELECT * FROM read_csv('/etc/passwd')")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "table function")
}
