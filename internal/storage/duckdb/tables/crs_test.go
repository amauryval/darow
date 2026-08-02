package tables

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_ParseGeometryCRS(t *testing.T) {
	cases := []struct {
		colType  string
		wantCRS  string
		wantSRID int
		wantOK   bool
	}{
		{colType: "GEOMETRY('EPSG:2154')", wantCRS: "EPSG:2154", wantSRID: 2154, wantOK: true},
		{colType: "GEOMETRY('OGC:CRS84')", wantCRS: "OGC:CRS84", wantSRID: 4326, wantOK: true},
		{colType: "GEOMETRY('BOGUS')", wantCRS: "BOGUS", wantSRID: 0, wantOK: true},
		{colType: "GEOMETRY", wantCRS: "", wantSRID: 0, wantOK: false},
		{colType: "VARCHAR", wantCRS: "", wantSRID: 0, wantOK: false},
	}

	for _, c := range cases {
		t.Run(c.colType, func(t *testing.T) {
			crs, srid, ok := parseGeometryCRS(c.colType)
			assert.Equal(t, c.wantCRS, crs)
			assert.Equal(t, c.wantSRID, srid)
			assert.Equal(t, c.wantOK, ok)
		})
	}
}
func Test_Materializer_DetectCRS(t *testing.T) {
	conn := openRepoTestDB(t)
	l := NewMaterializer(conn, conn, fakePhysicalNames{})

	authName, authCode, err := l.DetectCRS(context.Background(), lambert93Fixture(t), "")

	require.NoError(t, err)
	assert.Equal(t, "EPSG", authName)
	assert.Equal(t, 2154, authCode)
}

func Test_Materializer_DetectCRS_MultiLayer(t *testing.T) {
	conn := openRepoTestDB(t)
	l := NewMaterializer(conn, conn, fakePhysicalNames{})

	authName, authCode, err := l.DetectCRS(context.Background(), multiLayerGpkgFixture, "layer_b")

	require.NoError(t, err)
	assert.Equal(t, "EPSG", authName)
	assert.Equal(t, 4326, authCode)
}

func Test_Materializer_DetectCRS_NoRowsWhenLayerUnknown(t *testing.T) {
	conn := openRepoTestDB(t)
	l := NewMaterializer(conn, conn, fakePhysicalNames{})

	authName, authCode, err := l.DetectCRS(context.Background(), pointsGeoJSONFixture(t), "no-such-layer")

	require.NoError(t, err)
	assert.Empty(t, authName)
	assert.Zero(t, authCode, "an unresolvable layer reports no CRS rather than erroring")
}
