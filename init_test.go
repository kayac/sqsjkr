package sqsjkr

import (
	"fmt"
	"sync"

	"github.com/kayac/sqsjkr/throttle"
)

var (
	lm = new(sync.Mutex)
	tm = new(sync.Mutex)
)

type TestLocker struct {
	lockTable map[string]bool
}

func (tjl *TestLocker) Lock(k, v string) error {
	lm.Lock()
	defer lm.Unlock()

	if tjl.lockTable[k] {
		return fmt.Errorf("already locked")
	}

	tjl.lockTable[k] = true
	return nil
}
func (tjl *TestLocker) Unlock(k string) error {
	lm.Lock()
	defer lm.Unlock()

	tjl.lockTable[k] = false
	return nil
}

type TestThrottle struct {
	table map[string]bool
}

func (th *TestThrottle) Set(id string) error {
	tm.Lock()
	defer tm.Unlock()

	if th.table[id] {
		return throttle.ErrDuplicatedMessage
	}

	th.table[id] = true
	return nil
}

func (th *TestThrottle) Unset(id string) error {
	tm.Lock()
	th.table[id] = false
	tm.Unlock()
	return nil
}

func init() {
	logger = NewLogger()
	logger.SetLevel("debug")
}
