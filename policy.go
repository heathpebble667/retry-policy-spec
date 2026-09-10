// Package retrypolicy parses and pretty-prints a small text format for
// describing retry policies: how many attempts to make, how the delay
// between attempts grows, and which failures are worth retrying at all.
//
// The point of the format is to keep this decision out of code and in a
// config file or database column where it can be reviewed and changed
// without a deploy.
package retrypolicy

import "time"

// BackoffKind selects how the delay between attempts grows.
type BackoffKind string

const (
	BackoffFixed       BackoffKind = "fixed"
	BackoffLinear      BackoffKind = "linear"
	BackoffExponential BackoffKind = "exponential"
)

// Jitter selects how much randomness is mixed into each delay. Without
// jitter, every client backed off by the same policy retries in lockstep,
// which turns a brief blip into a synchronized wave of retries against the
// service that's already struggling.
type Jitter string

const (
	JitterNone  Jitter = "none"
	JitterFull  Jitter = "full"
	JitterEqual Jitter = "equal"
)

var validJitter = map[Jitter]bool{JitterNone: true, JitterFull: true, JitterEqual: true}

// Backoff describes the delay curve for one policy. Only the fields that
// apply to Kind are populated; see README.md for the parameter list of
// each kind.
type Backoff struct {
	Kind       BackoffKind
	Delay      time.Duration // fixed
	Base       time.Duration // linear, exponential
	Increment  time.Duration // linear
	Max        time.Duration // linear, exponential
	Multiplier float64       // exponential
}

// Policy is a fully validated retry policy. The zero value is not
// meaningful on its own; build one with Parse.
type Policy struct {
	MaxAttempts int
	Backoff     Backoff
	Jitter      Jitter

	// RetryOn lists the conditions worth retrying: HTTP status codes as
	// decimal strings ("503"), named conditions ("timeout",
	// "connection-error"), or the single entry "any". A nil slice means
	// the source didn't specify any, which callers should treat as "use
	// your own judgment" rather than "retry nothing".
	RetryOn []string
}
