package rest

import (
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/amauryval/darow/internal/darow/settings"
)

func Test_Settings_Global(t *testing.T) {
	h := setupRouter(t)

	require.Equal(t, http.StatusOK, send(t, h, http.MethodPut, "/settings", `{"featuresPageSize":2}`).Code)

	got := decodeBody[settings.Effective](t, get(t, h, "/settings", nil))
	assert.Equal(t, 2, got.FeaturesPageSize)
	page := decodeBody[FeatureCollection](t, get(t, h, ogcRoot+"/collections/places/items", nil))
	assert.Equal(t, 2, page.NumberReturned)
	assert.Equal(t, http.StatusUnprocessableEntity, send(t, h, http.MethodPut, "/settings", `{"featuresPageSize":0}`).Code)
}

func Test_Settings_Table(t *testing.T) {
	cases := []struct {
		name       string
		table      string
		body       string
		wantStatus int
	}{
		{name: "Overrides a table's settings", table: "places", body: `{"rowsPageSize":10,"tileCacheSeconds":60}`, wantStatus: http.StatusOK},
		{name: "An unknown table is not found", table: "nowhere", body: `{}`, wantStatus: http.StatusNotFound},
		{name: "Refuses a value out of its bounds", table: "places", body: `{"rowsPageSize":5000}`, wantStatus: http.StatusUnprocessableEntity},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			h := setupRouter(t)

			rec := send(t, h, http.MethodPut, "/tables/"+c.table+"/settings", c.body)

			require.Equal(t, c.wantStatus, rec.Code)
			if c.wantStatus != http.StatusOK {
				return
			}
			got := decodeBody[settings.TableSettings](t, rec)
			assert.Equal(t, 10, got.Effective.RowsPageSize)
			tile := get(t, h, "/tables/places/tiles/0/0/0", nil)
			assert.Equal(t, "public, max-age=60", tile.Header().Get("Cache-Control"))
		})
	}
}

func Test_Settings_DecimalsOfAnItem(t *testing.T) {
	h := setupRouter(t)
	require.Equal(t, http.StatusOK, send(t, h, http.MethodPut, "/tables/places/settings", `{"coordinateDecimals":0}`).Code)

	rec := get(t, h, ogcRoot+"/collections/places/items/1", nil)

	assert.True(t, strings.Contains(rec.Body.String(), `"coordinates":[2.0,49.0]`) || strings.Contains(rec.Body.String(), `"coordinates":[2,49]`), rec.Body.String())
}
