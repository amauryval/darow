package settings

// FieldDescriptor states the operational range of one runtime-editable
// setting and the unit its value is expressed in. A nil bound is one the
// field does not have.
type FieldDescriptor struct {
	Unit             string
	Minimum          *float64
	ExclusiveMinimum *float64
	Maximum          *float64
}

// Field names, the JSON keys of the settings routes and the keys of
// Descriptors.
const (
	FieldDefaultRowLimit      = "defaultRowLimit"
	FieldDefaultListLimit     = "defaultListLimit"
	FieldDefaultFeaturesLimit = "defaultFeaturesLimit"
	FieldMaxSimplifyTolerance = "maxSimplifyTolerance"
	FieldMaxPrecision         = "maxPrecision"
	FieldTileCacheMaxAgeSec   = "tileCacheMaxAgeSec"
	FieldCacheTTLSec          = "cacheTTLSec"
	FieldCoordinateDecimals   = "coordinateDecimals"
)

func bound(v float64) *float64 { return &v }

// Descriptors returns the descriptor of every runtime-editable setting,
// keyed by field name, off the very constants Validate enforces.
func Descriptors() map[string]FieldDescriptor {
	return map[string]FieldDescriptor{
		FieldDefaultRowLimit: {
			Unit:    "rows",
			Minimum: bound(LimitFloor),
			Maximum: bound(RowLimitCeiling),
		},
		FieldDefaultListLimit: {
			Unit:    "tables",
			Minimum: bound(LimitFloor),
			Maximum: bound(ListLimitCeiling),
		},
		FieldDefaultFeaturesLimit: {
			Unit:    "features",
			Minimum: bound(LimitFloor),
			Maximum: bound(FeaturesLimitCeiling),
		},
		FieldMaxSimplifyTolerance: {
			Unit:             "m",
			ExclusiveMinimum: bound(0),
		},
		FieldMaxPrecision: {
			Unit:             "m",
			ExclusiveMinimum: bound(0),
		},
		FieldTileCacheMaxAgeSec: {
			Unit:    "s",
			Minimum: bound(0),
		},
		FieldCacheTTLSec: {
			Unit:    "s",
			Minimum: bound(0),
		},
		FieldCoordinateDecimals: {
			Unit:    "digits",
			Minimum: bound(0),
			Maximum: bound(CoordinateDecimalsCeiling),
		},
	}
}

// Descriptor returns the descriptor of one field, and whether that field is
// runtime-editable at all.
func Descriptor(field string) (FieldDescriptor, bool) {
	d, ok := Descriptors()[field]
	return d, ok
}
