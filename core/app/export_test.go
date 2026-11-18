package app

import "time"

// SetStallAfter makes an import's step end as failed after d with nothing new, for a test,
// and returns what puts it back.
func SetStallAfter(d time.Duration) func() {
	was := stallAfter
	stallAfter = d
	return func() { stallAfter = was }
}
