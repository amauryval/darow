package engine

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/duckdb/duckdb-go/v2"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/MyAppLibs/go-pkg/apperr"
)

func Test_where(t *testing.T) {
	cases := []struct {
		name      string
		condition string
		want      string
		wantErr   error
	}{
		{name: "Returns nothing for a blank condition", condition: " "},
		{name: "Wraps the condition", condition: `"pop" > 1 OR "name" = 'Lyon'`, want: ` WHERE ("pop" > 1 OR "name" = 'Lyon')`},
		{name: "Refuses an unknown column", condition: "nope = 1", wantErr: apperr.ErrInvalid},
		{name: "Refuses a union out of the parentheses", condition: "1) UNION SELECT content FROM read_text('/etc/hostname') --", wantErr: apperr.ErrInvalid},
		{name: "Refuses a comment", condition: "true /* x */", wantErr: apperr.ErrInvalid},
		{name: "Refuses another source", condition: `1) FROM "places" WHERE (1`, wantErr: apperr.ErrInvalid},
	}
	f := setupEngine(t)
	f.places(t)
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var got string
			err := f.engine.read(context.Background(), func(db *sql.DB) error {
				table, err := describe(context.Background(), db, "places")
				require.NoError(t, err)
				got, err = where(context.Background(), db, table, c.condition)

				return err
			})

			if c.wantErr != nil {
				assert.ErrorIs(t, err, c.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, c.want, got)
		})
	}
}

func Test_evaluated(t *testing.T) {
	other := errors.New("disk on fire")
	cases := []struct {
		name        string
		err         error
		wantInvalid bool
	}{
		{name: "A value failing a cast is the user's", err: &duckdb.Error{Type: duckdb.ErrorTypeConversion, Msg: "Conversion Error"}, wantInvalid: true},
		{name: "A value out of range is the user's", err: &duckdb.Error{Type: duckdb.ErrorTypeOutOfRange, Msg: "Out of Range Error"}, wantInvalid: true},
		{name: "An interruption stays itself", err: &duckdb.Error{Type: duckdb.ErrorTypeInterrupt, Msg: "INTERRUPT Error"}},
		{name: "Another failure stays itself", err: other},
		{name: "No failure stays none"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := evaluated("where", c.err)

			assert.Equal(t, c.wantInvalid, errors.Is(err, apperr.ErrInvalid))
			if !c.wantInvalid {
				assert.Equal(t, c.err, err)
			}
		})
	}
}
