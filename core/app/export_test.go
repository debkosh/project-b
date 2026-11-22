package app

import (
	"time"

	"projectb/passwords"
)

// SetStallAfter makes an import's step end as failed after d with nothing new, for a test,
// and returns what puts it back.
func SetStallAfter(d time.Duration) func() {
	was := stallAfter
	stallAfter = d
	return func() { stallAfter = was }
}

// SetPasswordCheckHolds sets how long the device's check holds for Settings' Passwords, and
// returns what puts it back.
func SetPasswordCheckHolds(d time.Duration) func() {
	was := passwordCheckHolds
	passwordCheckHolds = d
	return func() { passwordCheckHolds = was }
}

// SetPasswordSettle sets how long a page a tab went on to has to show a sign-in again
// before the one sent is taken to have gone through, and returns what puts it back.
func SetPasswordSettle(d time.Duration) func() {
	was := passwords.Settle
	passwords.Settle = d
	return func() { passwords.Settle = was }
}
