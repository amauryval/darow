package router

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
)

func Test_MountUI(t *testing.T) {
	assets := fstest.MapFS{
		"index.html":       {Data: []byte("index")},
		"assets/app.js":    {Data: []byte("app-js")},
		"assets/style.css": {Data: []byte("app-css")},
	}
	handler := mountUI(assets)

	cases := []struct {
		name     string
		path     string
		wantBody string
	}{
		{name: "root serves index", path: "/ui/", wantBody: "index"},
		{name: "existing asset served as-is", path: "/ui/assets/app.js", wantBody: "app-js"},
		{name: "unknown SPA route falls back to index", path: "/ui/tables/some-id", wantBody: "index"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, c.path, nil)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			assert.Equal(t, http.StatusOK, rec.Code)
			assert.Equal(t, c.wantBody, rec.Body.String())
		})
	}
}
