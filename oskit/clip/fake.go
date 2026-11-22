package clip

import "sync"

// Fake is a Clipboard for tests: it keeps the secrets copied, in order.
type Fake struct {
	mu     sync.Mutex
	copied []string
}

var _ Clipboard = (*Fake)(nil)

func (f *Fake) CopySecret(text string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.copied = append(f.copied, text)
}

// Copied is every secret copied, in order.
func (f *Fake) Copied() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.copied...)
}
