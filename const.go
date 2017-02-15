package sqsjkr

import (
	"time"
)

// Default Const
const (
	DefaultMaxCocurrentNum = 20
	VisibilityTimeout      = 30
	WaitTimeSec            = 10
	MaxRetrieveMessageNum  = 10
	JobRetryInterval       = time.Second * 5
	ApplicationJSON        = "application/json"
	DefaultStatsPort       = 8061
)
