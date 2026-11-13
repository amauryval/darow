// Package modules lists the modules darow hosts, once for its binaries.
package modules

import (
	"github.com/MyAppLibs/go-pkg/starter"

	"github.com/amauryval/darow/internal/darow"
)

// RegisterFlags declares the flags of every module, in the order the starter opens them, before
// flag.Parse.
func RegisterFlags() []starter.ModuleFlags {
	return []starter.ModuleFlags{darow.RegisterFlags()}
}
