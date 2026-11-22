package secret

import (
	"context"
	"crypto/rand"
	"sync"
)

// Fake is a Store for tests: a key per name, kept in memory, made when asked to create.
type Fake struct {
	mu   sync.Mutex
	keys map[string][]byte
}

var _ Store = (*Fake)(nil)

func (f *Fake) DeviceKey(_ context.Context, name string, create bool) ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if k, ok := f.keys[name]; ok {
		return k, nil
	}
	if !create {
		return nil, ErrNone
	}
	if f.keys == nil {
		f.keys = map[string][]byte{}
	}
	k := make([]byte, 32)
	rand.Read(k)
	f.keys[name] = k
	return k, nil
}
