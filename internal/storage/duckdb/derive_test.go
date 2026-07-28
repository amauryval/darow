package duckdb

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_Adapter_ValidateDeriveQuery(t *testing.T) {
	a := setupAdapter(t)

	_, err := a.Exec(`CREATE TABLE t_a1b2c3 (__pkey TEXT, height INTEGER)`)
	require.NoError(t, err)
	_, err = a.Exec(`INSERT INTO metadata (tableId, physicalName, tableName) VALUES ('id-1', 't_a1b2c3', 'widgets')`)
	require.NoError(t, err)

	cases := []struct {
		name    string
		query   string
		wantErr string
	}{
		{name: "select over a registered table is allowed", query: "SELECT * FROM t_a1b2c3 WHERE height > 10"},
		{name: "cte referencing a registered table is allowed", query: "WITH x AS (SELECT * FROM t_a1b2c3) SELECT * FROM x"},
		{
			name:    "unregistered table is rejected",
			query:   "SELECT * FROM t_unknown",
			wantErr: `unknown table "t_unknown"`,
		},
		{
			name:    "metadata table is rejected",
			query:   "SELECT * FROM metadata",
			wantErr: `unknown table "metadata"`,
		},
		{
			name:    "file-reading table function is rejected",
			query:   "SELECT * FROM read_csv('/etc/passwd')",
			wantErr: `table function "read_csv" is not allowed`,
		},
		{
			name:    "system table function is rejected",
			query:   "SELECT * FROM duckdb_settings()",
			wantErr: `table function "duckdb_settings" is not allowed`,
		},
		{
			name:    "file-reading table function nested in a subquery is rejected",
			query:   "SELECT * FROM (SELECT * FROM read_text('/etc/hostname')) sub",
			wantErr: `table function "read_text" is not allowed`,
		},
		{
			name:    "non select statement is rejected",
			query:   "INSERT INTO t_a1b2c3 VALUES ('x', 1)",
			wantErr: "only a single SELECT statement is allowed",
		},
		{
			name:    "chained statements are rejected",
			query:   "SELECT * FROM t_a1b2c3; SELECT * FROM t_a1b2c3",
			wantErr: "single statement",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := a.validateDeriveQuery(c.query)
			if c.wantErr == "" {
				assert.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), c.wantErr)
		})
	}
}

func Test_Adapter_CreateFromQuery_RejectsUnregisteredTable(t *testing.T) {
	a := setupAdapter(t)

	_, _, err := a.CreateFromQuery("t_derived", "SELECT * FROM read_csv('/etc/passwd')")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "table function")
}
