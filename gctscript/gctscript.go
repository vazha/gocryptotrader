package gctscript

import (
	"github.com/vazha/gocryptotrader/gctscript/modules"
	"github.com/vazha/gocryptotrader/gctscript/wrappers/gct"
)

// Setup configures the wrapper interface to use
func Setup() {
	modules.SetModuleWrapper(gct.Setup())
}
