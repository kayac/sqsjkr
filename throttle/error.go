package throttle

import (
	"errors"
)

// throttle errors
var (
	ErrDuplicatedMessage = errors.New("duplicated message id")
)
