package router

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_newOpsHandler(t *testing.T) {
	ready := func(context.Context) error { return nil }

	oh := newOpsHandler(ready)

	assert.NoError(t, oh.ready(context.Background()))
}

func Test_opsHandler_healthz(t *testing.T) {
	oh := &opsHandler{}

	rec := doHandlerRequest(oh.healthz, http.MethodGet, "/healthz", nil, nil)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func Test_opsHandler_readyz(t *testing.T) {
	cases := []struct {
		name       string
		readyErr   error
		wantStatus int
	}{
		{name: "storage reachable returns 200", wantStatus: http.StatusOK},
		{name: "storage unreachable returns 503", readyErr: errors.New("duckdb unreachable"), wantStatus: http.StatusServiceUnavailable},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			oh := &opsHandler{ready: func(context.Context) error { return c.readyErr }}

			rec := doHandlerRequest(oh.readyz, http.MethodGet, "/readyz", nil, nil)

			assert.Equal(t, c.wantStatus, rec.Code)
		})
	}
}
