package router

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"darow/internal/router/response"
)

func Test_recoverJSON(t *testing.T) {
	t.Run("turns a panic into the standard error envelope", func(t *testing.T) {
		panicking := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			panic("boom")
		})

		req := httptest.NewRequest(http.MethodGet, "/tables", nil)
		rec := httptest.NewRecorder()

		recoverJSON(panicking).ServeHTTP(rec, req)

		assert.Equal(t, http.StatusInternalServerError, rec.Code)
		var body response.ErrorResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
		assert.Equal(t, http.StatusInternalServerError, body.Code)
	})

	t.Run("re-panics http.ErrAbortHandler untouched", func(t *testing.T) {
		aborting := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			panic(http.ErrAbortHandler)
		})

		req := httptest.NewRequest(http.MethodGet, "/tables", nil)
		rec := httptest.NewRecorder()

		assert.PanicsWithValue(t, http.ErrAbortHandler, func() {
			recoverJSON(aborting).ServeHTTP(rec, req)
		})
	})

	t.Run("does not recover on a healthy handler", func(t *testing.T) {
		ok := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		req := httptest.NewRequest(http.MethodGet, "/tables", nil)
		rec := httptest.NewRecorder()

		recoverJSON(ok).ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
	})
}

func Test_notFoundJSON(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/no-such-route", nil)
	rec := httptest.NewRecorder()

	notFoundJSON(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
	var body response.ErrorResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, "not_found", body.Error)
}

func Test_methodNotAllowedJSON(t *testing.T) {
	req := httptest.NewRequest(http.MethodPut, "/api/v1/tables", nil)
	rec := httptest.NewRecorder()

	methodNotAllowedJSON(rec, req)

	assert.Equal(t, http.StatusMethodNotAllowed, rec.Code)
	var body response.ErrorResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, http.StatusMethodNotAllowed, body.Code)
}
