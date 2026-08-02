package handlers

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"darow/internal/storage"
)

func Test_RowToJSON(t *testing.T) {
	t.Run("flattens pkey alongside data columns", func(t *testing.T) {
		row := storage.Row{Pkey: "pk-1", Values: map[string]any{"height": 12}}

		out := rowToJSON(row)

		assert.Equal(t, map[string]any{"pkey": "pk-1", "height": 12}, out)
	})

	t.Run("a data column literally named pkey never overwrites the row's real primary key", func(t *testing.T) {
		row := storage.Row{Pkey: "pk-1", Values: map[string]any{"pkey": "not-the-real-one", "height": 12}}

		out := rowToJSON(row)

		assert.Equal(t, map[string]any{"pkey": "pk-1", "height": 12}, out)
	})
}
