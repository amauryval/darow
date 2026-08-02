package validation

import (
	"context"

	"github.com/stretchr/testify/mock"

	"darow/internal/service"
	"darow/internal/storage"
)

type mockMetadataGetter struct{ mock.Mock }

var _ MetadataGetter = (*mockMetadataGetter)(nil)

func (m *mockMetadataGetter) GetMetadata(ctx context.Context, tableId string) (storage.Metadata, error) {
	args := m.Called(ctx, tableId)
	meta, _ := args.Get(0).(storage.Metadata)
	return meta, args.Error(1)
}

type mockSchemaGetter struct{ mock.Mock }

var _ SchemaGetter = (*mockSchemaGetter)(nil)

func (m *mockSchemaGetter) Schema(ctx context.Context, meta storage.Metadata) ([]service.ColumnSchema, error) {
	args := m.Called(ctx, meta)
	schema, _ := args.Get(0).([]service.ColumnSchema)
	return schema, args.Error(1)
}
