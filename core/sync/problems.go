package sync

import (
	"errors"
	"fmt"
	"mime"
	"net/http"
)

// What can go wrong reaching a bucket, each of which the settings page words its own way
// (Problem). A request's error wraps one of these when the server couldn't be reached, didn't
// answer as a bucket, or refused the token.
var (
	// ErrAddress is an address Propose doesn't take.
	ErrAddress = errors.New("sync: not a sync address")
	// ErrNeedsToken is an address that needs its token, given none.
	ErrNeedsToken = errors.New("sync: the address needs its token")
	// ErrUnreachable is a request that got no answer, or a server's own failure.
	ErrUnreachable = errors.New("sync: couldn't reach the server")
	// ErrNotBucket is an answer that isn't a bucket's: not found, or not JSON.
	ErrNotBucket = errors.New("sync: the address isn't a sync bucket")
	// ErrToken is the server refusing the token: it signed this device out.
	ErrToken = errors.New("sync: the server didn't accept the token")
)

// Problem names what err is for the settings page: "address", "needsToken", "unreachable",
// "notBucket", "token", or "" for anything else.
func Problem(err error) string {
	for _, p := range []struct {
		err  error
		name string
	}{{ErrAddress, "address"}, {ErrNeedsToken, "needsToken"}, {ErrUnreachable, "unreachable"}, {ErrNotBucket, "notBucket"}, {ErrToken, "token"}} {
		if errors.Is(err, p.err) {
			return p.name
		}
	}
	return ""
}

// checked is c with every answer looked at first: what the server's answer says of the bucket
// becomes one of the errors above, so a call's error says which.
func checked(c *http.Client) *http.Client {
	out := *c
	base := c.Transport
	if base == nil {
		base = http.DefaultTransport
	}
	out.Transport = checking{base}
	return &out
}

type checking struct{ base http.RoundTripper }

func (t checking) RoundTrip(r *http.Request) (*http.Response, error) {
	resp, err := t.base.RoundTrip(r)
	if err != nil {
		// A run cut short is no problem of the server's.
		if r.Context().Err() != nil {
			return nil, err
		}
		return nil, fmt.Errorf("%w: %v", ErrUnreachable, err)
	}
	var problem error
	switch kind, _, _ := mime.ParseMediaType(resp.Header.Get("Content-Type")); {
	case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
		problem = ErrToken
	case resp.StatusCode >= 500:
		problem = fmt.Errorf("%w: %s", ErrUnreachable, resp.Status)
	case resp.StatusCode == http.StatusNotFound || kind != "application/json":
		problem = fmt.Errorf("%w: %s, %s", ErrNotBucket, resp.Status, kind)
	default:
		return resp, nil
	}
	resp.Body.Close()
	return nil, problem
}
