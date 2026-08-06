package router

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"darow/internal/router/response"
)

func Test_maxBodyBytes(t *testing.T) {
	limit := int64(10)
	ok := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		_, err := io.ReadAll(req.Body)
		if err != nil {
			response.WriteError(w, req, err, http.StatusRequestEntityTooLarge)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	handler := maxBodyBytes(func() int64 { return limit })(ok)

	t.Run("rejects a declared Content-Length over the limit before reading the body", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("this body is over ten bytes"))
		req.ContentLength = 28
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusRequestEntityTooLarge, rec.Code)
	})

	t.Run("allows a body within the limit", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("short"))
		req.ContentLength = 5
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("reads limit fresh on every request", func(t *testing.T) {
		limit = 3
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("short"))
		req.ContentLength = 5
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusRequestEntityTooLarge, rec.Code)
	})
}

func Test_requestTimeout(t *testing.T) {
	t.Run("bounds the request context when secs is positive", func(t *testing.T) {
		var hadDeadline bool
		next := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			_, hadDeadline = req.Context().Deadline()
		})
		handler := requestTimeout(func() int { return 30 })(next)

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		assert.True(t, hadDeadline)
	})

	t.Run("leaves the request unbounded when secs is not positive", func(t *testing.T) {
		var hadDeadline bool
		next := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			_, hadDeadline = req.Context().Deadline()
		})
		handler := requestTimeout(func() int { return 0 })(next)

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		assert.False(t, hadDeadline)
	})
}

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
