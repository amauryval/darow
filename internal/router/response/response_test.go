package response

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_WriteError(t *testing.T) {
	cases := []struct {
		name        string
		code        int
		err         error
		wantMessage string
	}{
		{
			name:        "4xx keeps the underlying error detail",
			code:        http.StatusBadRequest,
			err:         errors.New("newTableName is required"),
			wantMessage: "newTableName is required",
		},
		{
			name:        "5xx replaces the detail with the generic status text",
			code:        http.StatusInternalServerError,
			err:         errors.New("duckdb: table t_a1b2c3 not found"),
			wantMessage: http.StatusText(http.StatusInternalServerError),
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/tables/id-1", nil)
			rec := httptest.NewRecorder()

			WriteError(rec, req, c.err, c.code)

			assert.Equal(t, c.code, rec.Code)
			var body ErrorResponse
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
			assert.Equal(t, c.wantMessage, body.Error)
			assert.Equal(t, c.code, body.Code)
		})
	}
}
