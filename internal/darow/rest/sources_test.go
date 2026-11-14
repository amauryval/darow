package rest

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/MyAppLibs/go-pkg/apperr"
)

func upload(t *testing.T, h http.Handler, field, name, content string) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile(field, name)
	require.NoError(t, err)
	_, err = part.Write([]byte(content))
	require.NoError(t, err)
	require.NoError(t, writer.Close())
	req := httptest.NewRequest(http.MethodPost, "/sources", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	return rec
}

func Test_UploadSource(t *testing.T) {
	cases := []struct {
		name       string
		field      string
		file       string
		content    string
		wantStatus int
		wantCode   apperr.Code
	}{
		{name: "Saves the file part", field: "file", file: "places.geojson", content: placesGeoJSON, wantStatus: http.StatusCreated},
		{name: "Refuses another part", field: "other", file: "places.geojson", content: placesGeoJSON, wantStatus: http.StatusBadRequest, wantCode: apperr.CodeInvalidBody},
		{name: "Refuses a file past its limit", field: "file", file: "big.csv", content: strings.Repeat("a", 2<<20), wantStatus: http.StatusUnprocessableEntity, wantCode: apperr.CodeInvalid},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rec := upload(t, setupRouter(t), c.field, c.file, c.content)

			require.Equal(t, c.wantStatus, rec.Code)
			if c.wantCode != "" {
				assert.Equal(t, c.wantCode, decodeBody[apperr.Problem](t, rec).Code)
			}
		})
	}
}
