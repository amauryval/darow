package router

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"

	"github.com/go-chi/chi/v5"
)

func newHandlerRequest(method, target string, body any, params map[string]string) *http.Request {
	var reader *bytes.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		reader = bytes.NewReader(b)
	} else {
		reader = bytes.NewReader(nil)
	}

	req := httptest.NewRequest(method, target, reader)
	if len(params) > 0 {
		rctx := chi.NewRouteContext()
		for k, v := range params {
			rctx.URLParams.Add(k, v)
		}
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	}
	return req
}

func doHandlerRequest(handlerFunc http.HandlerFunc, method, target string, body any, params map[string]string) *httptest.ResponseRecorder {
	req := newHandlerRequest(method, target, body, params)
	rec := httptest.NewRecorder()
	handlerFunc(rec, req)
	return rec
}

func decodeJSON[T any](rec *httptest.ResponseRecorder) T {
	var v T
	_ = json.Unmarshal(rec.Body.Bytes(), &v)
	return v
}
