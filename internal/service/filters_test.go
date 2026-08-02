package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/golang-lru/v2/expirable"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"darow/internal/storage"
	"darow/internal/storage/mocks"
)

var filterTestMeta = storage.Metadata{TableId: "id-1"}

func setupFilterService(t *testing.T) (*FilterService, *mocks.Adapter, *mocks.TableRepository) {
	m := &mocks.Adapter{}
	tableRepo := &mocks.TableRepository{}
	schemaSvc := NewSchemaService(m, expirable.NewLRU[string, []ColumnSchema](1000, nil, 5*time.Minute))
	t.Cleanup(func() { m.AssertExpectations(t); tableRepo.AssertExpectations(t) })
	return NewFilterService(m, schemaSvc), m, tableRepo
}

// expectSchema primes the one schema read validateConditions performs.
func expectSchema(m *mocks.Adapter, tableRepo *mocks.TableRepository) {
	m.On("Table", filterTestMeta).Return(tableRepo, nil).Once()
	tableRepo.On("Schema", context.Background()).Return([]storage.ColumnInfo{
		{Name: "name", Type: "VARCHAR"},
		{Name: "height", Type: "DOUBLE"},
	}, nil).Once()
}

func Test_FilterService_Create(t *testing.T) {
	svc, m, tableRepo := setupFilterService(t)
	conditions := []storage.FilterCondition{{Column: "height", Operator: ">", Value: float64(10)}}
	stored := storage.StoredFilter{FilterId: "f-1", TableId: "id-1", Name: "Hauts", Conditions: conditions}

	expectSchema(m, tableRepo)
	m.On("CreateFilter", context.Background(), "id-1", "Hauts", conditions).Return(stored, nil).Once()

	got, err := svc.Create(context.Background(), filterTestMeta, "  Hauts  ", conditions)

	require.NoError(t, err)
	assert.Equal(t, stored, got)
}

func Test_FilterService_Create_RejectsInvalidInput(t *testing.T) {
	conditions := []storage.FilterCondition{{Column: "height", Operator: ">", Value: float64(10)}}

	cases := []struct {
		name        string
		filterName  string
		conditions  []storage.FilterCondition
		needsSchema bool
	}{
		{name: "blank name", filterName: "   ", conditions: conditions},
		{name: "overlong name", filterName: strings.Repeat("a", maxFilterNameLength+1), conditions: conditions},
		{name: "no condition", filterName: "Vide", conditions: nil},
		{
			name:        "unknown column",
			filterName:  "Inconnue",
			conditions:  []storage.FilterCondition{{Column: "nope", Operator: "=", Value: "x"}},
			needsSchema: true,
		},
		{
			name:        "unsupported operator",
			filterName:  "Mauvais opérateur",
			conditions:  []storage.FilterCondition{{Column: "name", Operator: "; DROP TABLE x", Value: "x"}},
			needsSchema: true,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			svc, m, tableRepo := setupFilterService(t)
			if c.needsSchema {
				expectSchema(m, tableRepo)
			}

			_, err := svc.Create(context.Background(), filterTestMeta, c.filterName, c.conditions)

			var validationErr *ValidationError
			assert.ErrorAs(t, err, &validationErr)
		})
	}
}

func Test_FilterService_Update_PatchesOnlyWhatItCarries(t *testing.T) {
	svc, m, _ := setupFilterService(t)
	stored := storage.StoredFilter{FilterId: "f-1", TableId: "id-1", Name: "Avant"}
	name := "Après"
	patch := storage.StoredFilterPatch{Name: &name}

	m.On("GetFilter", context.Background(), "f-1").Return(stored, nil).Once()
	m.On("UpdateFilter", context.Background(), "f-1", mock.MatchedBy(func(p storage.StoredFilterPatch) bool {
		return p.Conditions == nil && p.Name != nil && *p.Name == "Après"
	})).Return(storage.StoredFilter{FilterId: "f-1", TableId: "id-1", Name: "Après"}, nil).Once()

	got, err := svc.Update(context.Background(), filterTestMeta, "f-1", patch)

	require.NoError(t, err)
	assert.Equal(t, "Après", got.Name)
}

// A filter stored against another table is reported as missing rather than
// served, updated or deleted across tables.
func Test_FilterService_RejectsAFilterOfAnotherTable(t *testing.T) {
	other := storage.StoredFilter{FilterId: "f-1", TableId: "id-other"}

	t.Run("get", func(t *testing.T) {
		svc, m, _ := setupFilterService(t)
		m.On("GetFilter", context.Background(), "f-1").Return(other, nil).Once()

		_, err := svc.Get(context.Background(), filterTestMeta, "f-1")

		var notFound *NotFoundError
		assert.ErrorAs(t, err, &notFound)
	})

	t.Run("delete", func(t *testing.T) {
		svc, m, _ := setupFilterService(t)
		m.On("GetFilter", context.Background(), "f-1").Return(other, nil).Once()

		err := svc.Delete(context.Background(), filterTestMeta, "f-1")

		var notFound *NotFoundError
		assert.ErrorAs(t, err, &notFound)
	})
}

func Test_FilterService_Delete(t *testing.T) {
	svc, m, _ := setupFilterService(t)

	m.On("GetFilter", context.Background(), "f-1").
		Return(storage.StoredFilter{FilterId: "f-1", TableId: "id-1"}, nil).Once()
	m.On("DeleteFilter", context.Background(), "f-1").Return(nil).Once()

	assert.NoError(t, svc.Delete(context.Background(), filterTestMeta, "f-1"))
}

func Test_FilterService_List(t *testing.T) {
	svc, m, _ := setupFilterService(t)
	stored := []storage.StoredFilter{{FilterId: "f-1", TableId: "id-1", Name: "Un"}}

	m.On("ListFilters", context.Background(), "id-1").Return(stored, nil).Once()

	got, err := svc.List(context.Background(), filterTestMeta)

	require.NoError(t, err)
	assert.Equal(t, stored, got)
}
