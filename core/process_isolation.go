package core

import (
	"os"
	"runtime"

	"singbox-launcher/internal/constants"
)

// Isolated Linux bundles own only their tracked child. They must never offer
// name-based termination of the installed VPN or clean another app's adapters.
func isolatedCoreSession() bool {
	return runtime.GOOS == "linux" && os.Getenv(constants.EnvIsolatedCore) == "1"
}
