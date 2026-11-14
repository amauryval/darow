package settings

// Values are settings, each nil when not set: unset globally, it takes its default; unset for a
// table, the global value.
type Values struct {
	RowsPageSize       *int `json:"rowsPageSize"`
	FeaturesPageSize   *int `json:"featuresPageSize"`
	CoordinateDecimals *int `json:"coordinateDecimals"`
	TileCacheSeconds   *int `json:"tileCacheSeconds"`
}

// Effective are the settings applied to a table.
type Effective struct {
	RowsPageSize       int `json:"rowsPageSize"`
	FeaturesPageSize   int `json:"featuresPageSize"`
	CoordinateDecimals int `json:"coordinateDecimals"`
	TileCacheSeconds   int `json:"tileCacheSeconds"`
}

// TableSettings are a table's overrides and the settings applied to it.
type TableSettings struct {
	Overrides Values    `json:"overrides"`
	Effective Effective `json:"effective"`
}
