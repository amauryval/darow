package rest

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/amauryval/darow/internal/darow/engine"
)

func Test_Columns(t *testing.T) {
	cases := []struct {
		name       string
		method     string
		path       string
		body       string
		wantStatus int
	}{
		{name: "Adds a column", method: http.MethodPost, path: "/tables/places/columns", body: `{"name":"rank","type":"BIGINT"}`, wantStatus: http.StatusCreated},
		{name: "Refuses a type outside the list", method: http.MethodPost, path: "/tables/places/columns", body: `{"name":"rank","type":"BLOB"}`, wantStatus: http.StatusUnprocessableEntity},
		{name: "Renames a column", method: http.MethodPatch, path: "/tables/places/columns/name", body: `{"name":"label"}`, wantStatus: http.StatusOK},
		{name: "Deletes a column", method: http.MethodDelete, path: "/tables/places/columns/name", wantStatus: http.StatusOK},
		{name: "Refuses the geometry", method: http.MethodDelete, path: "/tables/places/columns/geom", wantStatus: http.StatusUnprocessableEntity},
		{name: "Previews a transform", method: http.MethodPost, path: "/tables/places/columns/name/transform/preview", body: `{"expression":"upper(name)"}`, wantStatus: http.StatusOK},
		{name: "Transforms a column", method: http.MethodPost, path: "/tables/places/columns/name/transform", body: `{"expression":"upper(name)","where":"\"name\" = 'Lyon'"}`, wantStatus: http.StatusOK},
		{name: "Refuses a subquery", method: http.MethodPost, path: "/tables/places/columns/name/transform", body: `{"expression":"(SELECT 1)"}`, wantStatus: http.StatusUnprocessableEntity},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.wantStatus, send(t, setupRouter(t), c.method, c.path, c.body).Code)
		})
	}
}

func Test_Columns_TransformWritten(t *testing.T) {
	h := setupRouter(t)

	rec := send(t, h, http.MethodPost, "/tables/places/columns/name/transform", `{"expression":"upper(name)"}`)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, int64(3), decodeBody[TransformResult](t, rec).Written)
	preview := decodeBody[engine.TransformPreview](t, send(t, h, http.MethodPost, "/tables/places/columns/name/transform/preview", `{"expression":"lower(name)"}`))
	assert.Equal(t, int64(3), preview.Matched)
}
