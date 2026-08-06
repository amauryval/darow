package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"darow/internal/storage"
	"darow/internal/storage/mocks"
)

func Test_DiagnosticsService_Diagnostics(t *testing.T) {
	m := &mocks.Adapter{}
	want := storage.Diagnostics{DatabaseSizeBytes: 1024, MemoryUsageBytes: 512, Extensions: []string{"spatial"}}
	m.On("Diagnostics", mock.Anything).Return(want, nil)

	svc := NewDiagnosticsService(m)
	got, err := svc.Diagnostics(context.Background())

	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func Test_DiagnosticsService_Diagnostics_WrapsStorageError(t *testing.T) {
	m := &mocks.Adapter{}
	m.On("Diagnostics", mock.Anything).Return(storage.Diagnostics{}, errors.New("duckdb unreachable"))

	svc := NewDiagnosticsService(m)
	_, err := svc.Diagnostics(context.Background())

	require.Error(t, err)
	var displayErr *DisplayError
	require.ErrorAs(t, err, &displayErr)
}
