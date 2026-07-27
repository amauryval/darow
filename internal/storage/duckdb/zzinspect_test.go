package duckdb

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_Inspect_Versions(t *testing.T) {
	a := setupAdapter(t)
	var v string
	require.NoError(t, a.QueryRow(`SELECT version()`).Scan(&v))
	t.Logf("duckdb version: %s", v)

	rows, err := a.Query(`SELECT extension_name, extension_version, installed FROM duckdb_extensions() WHERE extension_name = 'spatial'`)
	require.NoError(t, err)
	defer rows.Close()
	for rows.Next() {
		var name, ver string
		var installed bool
		require.NoError(t, rows.Scan(&name, &ver, &installed))
		t.Logf("extension: %s version=%s installed=%v", name, ver, installed)
	}
}
