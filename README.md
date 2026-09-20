# retry-policy-spec

Retry logic tends to end up hardcoded: a magic number of attempts, a
`time.Sleep(2 * time.Second)`, maybe an exponent someone tuned during an
incident two years ago and never revisited. Changing it means a deploy, and
nobody can look at one place to see what a given call actually does when it
fails.

This is a small text format for describing a retry policy, plus a Go
package that parses it with full validation and prints it back out in a
canonical form. The idea is to keep the policy in a config file, a database
column, or an env var, next to the things that actually vary between
environments and services.

There's no execution engine here — no sleeping, no HTTP client wrapper.
Just parsing a policy into a struct you can trust, and rendering it back
into text you can diff or store.

## Format

```
max_attempts: 5
backoff: exponential base=200ms max=30s multiplier=2.0
jitter: full
retry_on: 429, 502, 503, 504, timeout
```

- Lines are `key: value`. Blank lines and lines starting with `#` are
  ignored.
- `max_attempts` (required): an integer from 1 to 1000.
- `backoff` (required): a kind followed by `key=value` parameters.
  - `fixed delay=<duration>` — always wait the same amount of time.
  - `linear base=<duration> increment=<duration> max=<duration>` — wait
    increases by `increment` each attempt, capped at `max`.
  - `exponential base=<duration> max=<duration> multiplier=<float>` — wait
    is multiplied by `multiplier` each attempt, capped at `max`.
    `multiplier` must be greater than 1.
  - Durations use Go's duration syntax (`500ms`, `2s`, `1m30s`) and must be
    positive. `base` must be less than `max`.
- `jitter` (optional, default `none`): `none`, `full`, or `equal`. Jitter
  keeps many clients on the same policy from retrying in lockstep against a
  service that's already struggling.
- `retry_on` (optional): a comma-separated list of what's worth retrying —
  HTTP status codes in the 400-599 range, the named conditions `timeout`
  and `connection-error`, or the single entry `any`. Leaving it out means
  the source didn't say, which callers should treat as "decide for
  yourself", not "retry nothing".

Keys, kinds, and enum values are case sensitive and must be lowercase, and
every parameter is validated at parse time — a `*Policy` you get back from
`Parse` is always one that could actually be executed.

## Usage

```go
package main

import (
	"fmt"
	"log"

	retrypolicy "github.com/heathpebble667/retry-policy-spec"
)

const spec = `
max_attempts: 5
backoff: exponential base=200ms max=30s multiplier=2.0
jitter: full
retry_on: 429, 502, 503, 504, timeout
`

func main() {
	policy, err := retrypolicy.Parse(spec)
	if err != nil {
		log.Fatalf("invalid retry policy: %v", err)
	}

	for i, wait := range policy.Backoff.Delays(policy.MaxAttempts) {
		fmt.Printf("after attempt %d, wait %s\n", i+1, wait)
	}

	// Policy implements String(), so it round-trips through its own
	// canonical form.
	fmt.Print(policy)
}
```

A parse failure names the offending line:

```
line 3: backoff.multiplier: must be greater than 1, got 1
```

## Status

Early. The parser, printer, `Backoff.Delays` helper, and test suite exist.
Still missing: a CLI for validating and reformatting files in place, JSON
and text marshaling for interop with non-Go services and config loaders,
and fuzz testing of the parser.
