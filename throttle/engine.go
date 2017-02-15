package throttle

import (
	"time"
)

// Throttole const
const (
	DeleteTickerPeriod   = time.Hour * 1
	GlobalSecondaryIndex = "TypeExpiredIndex"
	DeleteCapacityRate   = 5
)

// Throttler struct
type Throttler interface {
	Set(id string) error
	Unset(id string) error
}

// DefaultThrottler do nothing
type DefaultThrottler struct{}

// Set DefaultThrotter
func (dl DefaultThrottler) Set(k string) error { return nil }

// Unset DefaultThrotter
func (dl DefaultThrottler) Unset(k string) error { return nil }
