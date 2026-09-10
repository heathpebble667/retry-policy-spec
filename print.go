package retrypolicy

import (
	"fmt"
	"strconv"
	"strings"
)

// String renders p in canonical form: one key per line, in a fixed order,
// with durations and numbers normalized. Parsing the output of String
// always yields a Policy equal to p.
func (p *Policy) String() string {
	var b strings.Builder
	fmt.Fprintf(&b, "max_attempts: %d\n", p.MaxAttempts)
	fmt.Fprintf(&b, "backoff: %s\n", formatBackoff(p.Backoff))
	fmt.Fprintf(&b, "jitter: %s\n", p.Jitter)
	if len(p.RetryOn) > 0 {
		fmt.Fprintf(&b, "retry_on: %s\n", strings.Join(p.RetryOn, ", "))
	}
	return b.String()
}

func formatBackoff(b Backoff) string {
	switch b.Kind {
	case BackoffFixed:
		return fmt.Sprintf("fixed delay=%s", b.Delay)
	case BackoffLinear:
		return fmt.Sprintf("linear base=%s increment=%s max=%s", b.Base, b.Increment, b.Max)
	case BackoffExponential:
		return fmt.Sprintf("exponential base=%s max=%s multiplier=%s", b.Base, b.Max, formatMultiplier(b.Multiplier))
	default:
		// Unreachable for a Policy built by Parse; kept so String never
		// panics on a hand-built Policy during development.
		return string(b.Kind)
	}
}

func formatMultiplier(f float64) string {
	return strconv.FormatFloat(f, 'g', -1, 64)
}
