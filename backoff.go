package retrypolicy

import "time"

// Delays returns the wait before each retry: the pause between the first
// attempt and the second, between the second and the third, and so on. Its
// length is attempts-1, since nothing waits before the first attempt. An
// attempts value of 1 or less needs no retries and returns nil.
//
// Callers with a Policy typically call policy.Backoff.Delays(policy.MaxAttempts).
func (b Backoff) Delays(attempts int) []time.Duration {
	if attempts <= 1 {
		return nil
	}
	delays := make([]time.Duration, attempts-1)

	switch b.Kind {
	case BackoffFixed:
		for i := range delays {
			delays[i] = b.Delay
		}

	case BackoffLinear:
		d := b.Base
		for i := range delays {
			delays[i] = d
			d += b.Increment
			if d > b.Max {
				d = b.Max
			}
		}

	case BackoffExponential:
		d := b.Base
		for i := range delays {
			delays[i] = d
			d = time.Duration(float64(d) * b.Multiplier)
			if d > b.Max {
				d = b.Max
			}
		}
	}

	return delays
}
