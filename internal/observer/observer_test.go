package observer

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_Observer_Notify(t *testing.T) {
	var o Observer[string]
	var first, second []string
	o.On(func(s string) { first = append(first, s) })
	o.On(func(s string) { second = append(second, s) })

	o.Notify("a")
	o.Notify("b")

	assert.Equal(t, []string{"a", "b"}, first)
	assert.Equal(t, []string{"a", "b"}, second)
}

func Test_Observer_Notify_NoListeners(t *testing.T) {
	var o Observer[string]
	assert.NotPanics(t, func() { o.Notify("a") })
}
