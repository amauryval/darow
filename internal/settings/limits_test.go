package settings

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_Limit_Resolve(t *testing.T) {
	cases := []struct {
		name      string
		requested int
		want      int
	}{
		{name: "nothing requested serves the stored page", requested: 0, want: 100},
		{name: "a negative request serves the stored page", requested: -1, want: 100},
		{name: "a smaller request is served as asked", requested: 60, want: 60},
		{name: "a request past the stored page is capped", requested: 5000, want: 100},
	}
	limit := Limit(100)
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, limit.Resolve(c.requested))
			assert.Equal(t, int64(c.want), limit.Resolve64(int64(c.requested)))
		})
	}
}
