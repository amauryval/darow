package handlers

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"darow/internal/storage"
)

func Test_parseGeoQuery(t *testing.T) {
	cases := []struct {
		name         string
		filter       string
		simplify     string
		precision    string
		validColumns []storage.ColumnInfo
		wantErr      bool
		wantFilter   string
		wantArgs     []any
		wantTol      *float64
		wantPrec     *float64
	}{
		{name: "no params returns empty query"},
		{
			name:         "valid filter builds a clause",
			filter:       `[{"column":"status","operator":"=","value":"a"}]`,
			validColumns: []storage.ColumnInfo{{Name: "status", Type: "VARCHAR"}},
			wantFilter:   `"status" = TRY_CAST(? AS VARCHAR)`,
			wantArgs:     []any{"a"},
		},
		{name: "invalid filter JSON is rejected", filter: `not-json`, wantErr: true},
		{name: "unknown filter column is rejected", filter: `[{"column":"bogus","operator":"=","value":"a"}]`, wantErr: true},
		{name: "valid simplify sets tolerance", simplify: "5", wantTol: ptr(5.0)},
		{name: "simplify above max is clamped", simplify: "1000", wantTol: ptr(100.0)},
		{name: "negative simplify is rejected", simplify: "-1", wantErr: true},
		{name: "non-numeric simplify is rejected", simplify: "abc", wantErr: true},
		{name: "valid precision sets precision", precision: "5", wantPrec: ptr(5.0)},
		{name: "precision above max is clamped", precision: "1000", wantPrec: ptr(50.0)},
		{name: "negative precision is rejected", precision: "-1", wantErr: true},
		{name: "non-numeric precision is rejected", precision: "abc", wantErr: true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			params := url.Values{}
			if c.filter != "" {
				params.Set("filter", c.filter)
			}
			if c.simplify != "" {
				params.Set("simplify", c.simplify)
			}
			if c.precision != "" {
				params.Set("precision", c.precision)
			}
			req := httptest.NewRequest(http.MethodGet, "/?"+params.Encode(), nil)

			q, err := parseGeoQuery(req, c.validColumns, 100, 50)

			if c.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, c.wantFilter, q.Filter)
			assert.Equal(t, c.wantArgs, q.Args)
			if c.wantTol == nil {
				assert.Nil(t, q.Tolerance)
			} else {
				require.NotNil(t, q.Tolerance)
				assert.Equal(t, *c.wantTol, *q.Tolerance)
			}
			if c.wantPrec == nil {
				assert.Nil(t, q.Precision)
			} else {
				require.NotNil(t, q.Precision)
				assert.Equal(t, *c.wantPrec, *q.Precision)
			}
		})
	}
}

func Test_parseBboxParam(t *testing.T) {
	cases := []struct {
		name     string
		bbox     string
		wantErr  bool
		wantBbox *[4]float64
	}{
		{name: "no bbox returns nil"},
		{name: "valid bbox parses", bbox: "1,2,3,4", wantBbox: &[4]float64{1, 2, 3, 4}},
		{name: "3D bbox drops its height ordinates", bbox: "1,2,10,3,4,20", wantBbox: &[4]float64{1, 2, 3, 4}},
		{name: "wrong number of parts is rejected", bbox: "1,2,3", wantErr: true},
		{name: "five parts are rejected", bbox: "1,2,3,4,5", wantErr: true},
		{name: "non-numeric part is rejected", bbox: "1,2,3,a", wantErr: true},
		{name: "inverted latitudes are rejected", bbox: "1,9,3,4", wantErr: true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			params := url.Values{}
			if c.bbox != "" {
				params.Set("bbox", c.bbox)
			}
			req := httptest.NewRequest(http.MethodGet, "/?"+params.Encode(), nil)

			bbox, err := parseBboxParam(req)

			if c.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, c.wantBbox, bbox)
		})
	}
}

func ptr(v float64) *float64 { return &v }
