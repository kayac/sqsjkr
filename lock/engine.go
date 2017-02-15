package lock

// Locker implements lock and unlock
type Locker interface {
	Lock(string, string) error
	Unlock(string) error
}

// DefaultLocker do nothing
type DefaultLocker struct{}

// Lock DefaultLocker
func (dl DefaultLocker) Lock(k, v string) error { return nil }

// Unlock DefaultLocker
func (dl DefaultLocker) Unlock(k string) error { return nil }
