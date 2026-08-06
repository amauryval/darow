package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"darow/internal/storage"
	"darow/internal/storage/mocks"
)

// allowSizeGuard is a sizeGuard double that always allows a write and
// counts how many times Grew was called — the guard RowService/
// ImportService/DeriveService's own tests use, since they exercise a
// disabled guard (duckdb.maxSizeBytes unset) and only need to assert the
// write itself.
type allowSizeGuard struct {
	err    error
	grewN  int
	checkN int
}

func (g *allowSizeGuard) Check(ctx context.Context) error {
	g.checkN++
	return g.err
}

func (g *allowSizeGuard) Grew() { g.grewN++ }

func Test_SizeGuardService_Check_DisabledByDefault(t *testing.T) {
	m := &mocks.Adapter{}
	svc := NewSizeGuardService(m, 0, time.Second)

	assert.NoError(t, svc.Check(context.Background()))
	m.AssertNotCalled(t, "Diagnostics", mock.Anything)
}

func Test_SizeGuardService_Check_AllowsUnderLimit(t *testing.T) {
	m := &mocks.Adapter{}
	m.On("Diagnostics", mock.Anything).Return(storage.Diagnostics{DatabaseSizeBytes: 100}, nil).Once()
	svc := NewSizeGuardService(m, 1000, time.Minute)

	assert.NoError(t, svc.Check(context.Background()))
}

func Test_SizeGuardService_Check_RefusesAtOrOverLimit(t *testing.T) {
	m := &mocks.Adapter{}
	m.On("Diagnostics", mock.Anything).Return(storage.Diagnostics{DatabaseSizeBytes: 1000}, nil).Once()
	svc := NewSizeGuardService(m, 1000, time.Minute)

	err := svc.Check(context.Background())

	require.Error(t, err)
	var quotaErr *QuotaExceededError
	require.ErrorAs(t, err, &quotaErr)
}

func Test_SizeGuardService_Check_CachesWithinTTL(t *testing.T) {
	m := &mocks.Adapter{}
	m.On("Diagnostics", mock.Anything).Return(storage.Diagnostics{DatabaseSizeBytes: 100}, nil).Once()
	svc := NewSizeGuardService(m, 1000, time.Minute)

	require.NoError(t, svc.Check(context.Background()))
	require.NoError(t, svc.Check(context.Background()))

	m.AssertNumberOfCalls(t, "Diagnostics", 1)
}

func Test_SizeGuardService_Grew_InvalidatesTheCache(t *testing.T) {
	m := &mocks.Adapter{}
	m.On("Diagnostics", mock.Anything).Return(storage.Diagnostics{DatabaseSizeBytes: 100}, nil).Twice()
	svc := NewSizeGuardService(m, 1000, time.Minute)

	require.NoError(t, svc.Check(context.Background()))
	svc.Grew()
	require.NoError(t, svc.Check(context.Background()))

	m.AssertNumberOfCalls(t, "Diagnostics", 2)
}

func Test_SizeGuardService_Check_WrapsStorageError(t *testing.T) {
	m := &mocks.Adapter{}
	m.On("Diagnostics", mock.Anything).Return(storage.Diagnostics{}, errors.New("duckdb unreachable"))
	svc := NewSizeGuardService(m, 1000, time.Minute)

	err := svc.Check(context.Background())

	require.Error(t, err)
	var displayErr *DisplayError
	require.ErrorAs(t, err, &displayErr)
}
