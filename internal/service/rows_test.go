package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gogeoserver/internal/storage"
)

func Test_ConvertValue(t *testing.T) {
	cases := []struct {
		name    string
		colType string
		value   any
		want    any
	}{
		{name: "date formatted as YYYY-MM-DD", colType: "DATE", value: time.Date(2024, 3, 15, 0, 0, 0, 0, time.UTC), want: "2024-03-15"},
		{name: "timestamp formatted as RFC3339", colType: "TIMESTAMP", value: time.Date(2024, 3, 15, 10, 30, 0, 0, time.UTC), want: "2024-03-15T10:30:00Z"},
		{name: "non time value passes through", colType: "INTEGER", value: 42, want: 42},
		{name: "string value passes through", colType: "VARCHAR", value: "hello", want: "hello"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := convertValue(c.colType, c.value)
			assert.Equal(t, c.want, got)
		})
	}
}

func Test_TableService_ListRows(t *testing.T) {
	svc, m := setupTableService(t)

	m.On("GetPkeys", "id-1", int64(0), int64(10)).Return([]string{"pk-1"}, nil).Once()
	m.On("GetRowsByPkey", "id-1", []string{"pk-1"}).Return([]storage.Row{
		{Pkey: "pk-1", Values: map[string]any{"createdAt": time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)}},
	}, nil).Once()
	m.On("ColumnType", "id-1", "createdAt").Return("DATE", nil).Once()
	m.On("RowsCount", "id-1").Return(int64(1), nil).Once()

	page, err := svc.ListRows("id-1", 0, 10)

	require.NoError(t, err)
	assert.Equal(t, int64(1), page.Total)
	require.Len(t, page.Rows, 1)
	assert.Equal(t, "2024-01-01", page.Rows[0].Values["createdAt"])
}

func Test_TableService_GetRow_NotFound(t *testing.T) {
	svc, m := setupTableService(t)

	m.On("GetRowsByPkey", "id-1", []string{"missing"}).Return([]storage.Row{}, nil).Once()

	row, err := svc.GetRow("id-1", "missing")

	require.NoError(t, err)
	assert.Nil(t, row)
}
