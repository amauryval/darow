package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_paginationParams(t *testing.T) {
	cases := []struct {
		name       string
		query      string
		strict     bool
		wantErr    bool
		wantOffset int64
		wantLimit  int64
	}{
		{name: "defaults when absent", query: "", wantOffset: 0, wantLimit: 50},
		{name: "parses explicit offset/limit", query: "offset=10&limit=20", wantOffset: 10, wantLimit: 20},
		{name: "negative offset falls back to zero", query: "offset=-1", wantOffset: 0, wantLimit: 50},
		{name: "non-positive limit falls back to default", query: "limit=0", wantOffset: 0, wantLimit: 50},
		{name: "limit is capped at the page size", query: "limit=9999", wantOffset: 0, wantLimit: 50},
		{name: "strict defaults when absent", query: "", strict: true, wantOffset: 0, wantLimit: 50},
		{name: "strict honors explicit offset/limit", query: "offset=10&limit=20", strict: true, wantOffset: 10, wantLimit: 20},
		{name: "strict rejects a negative offset", query: "offset=-1", strict: true, wantErr: true},
		{name: "strict rejects an unparseable offset", query: "offset=abc", strict: true, wantErr: true},
		{name: "strict rejects a non-positive limit", query: "limit=0", strict: true, wantErr: true},
		{name: "strict rejects an unparseable limit", query: "limit=abc", strict: true, wantErr: true},
		{name: "strict still caps a large limit", query: "limit=9999", strict: true, wantOffset: 0, wantLimit: 50},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/?"+c.query, nil)

			offset, limit, err := paginationParams(req, 50, c.strict)

			if c.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, c.wantOffset, offset)
			assert.Equal(t, c.wantLimit, limit)
		})
	}
}
