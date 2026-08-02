package service

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_DisplayError(t *testing.T) {
	cause := &NotFoundError{"unknown table"}
	err := NewDisplayError("impossible de charger la source", cause)

	assert.Equal(t, "impossible de charger la source", err.DisplayMessage())
	assert.Contains(t, err.Error(), "impossible de charger la source")
	assert.Contains(t, err.Error(), "unknown table")
	assert.Same(t, cause, err.Unwrap())

	var notFoundErr *NotFoundError
	assert.True(t, errors.As(err, &notFoundErr), "errors.As must still reach the wrapped cause through Unwrap")
	assert.Same(t, cause, notFoundErr)
}
