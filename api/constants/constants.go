package constants

import "time"

const (
	// DefaultFullSyncInterval is the default interval between 2 full-sync used by the
	// controller informers
	DefaultFullSyncInterval = 12 * time.Hour
)
