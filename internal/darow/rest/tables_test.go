package rest

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_Export_CRS(t *testing.T) {
	cases := []struct {
		name       string
		query      string
		wantStatus int
		contains   string
		wantFile   string
	}{
		{name: "Exports in CRS84 by default", query: "?format=csv", wantStatus: http.StatusOK, contains: "POINT (2.3", wantFile: "attachment; filename=places.csv"},
		{name: "Exports in the CRS asked for", query: "?format=csv&srid=2154", wantStatus: http.StatusOK, contains: "POINT (652301.5"},
		{name: "Refuses an unknown EPSG code", query: "?format=csv&srid=999999", wantStatus: http.StatusUnprocessableEntity},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rec := get(t, setupRouter(t), "/tables/places/export"+c.query, nil)

			require.Equal(t, c.wantStatus, rec.Code)
			assert.Contains(t, rec.Body.String(), c.contains)
			if c.wantFile != "" {
				assert.Equal(t, c.wantFile, rec.Header().Get("Content-Disposition"))
			}
		})
	}
}
