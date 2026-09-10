package retrypolicy

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// ParseError reports a problem with a specific line of policy source. Line
// is 0 when the problem isn't tied to one line, such as a missing key.
type ParseError struct {
	Line int
	Msg  string
}

func (e *ParseError) Error() string {
	if e.Line <= 0 {
		return e.Msg
	}
	return fmt.Sprintf("line %d: %s", e.Line, e.Msg)
}

func errf(line int, format string, args ...any) *ParseError {
	return &ParseError{Line: line, Msg: fmt.Sprintf(format, args...)}
}

var allowedKeys = map[string]bool{
	"max_attempts": true,
	"backoff":      true,
	"jitter":       true,
	"retry_on":     true,
}

var namedConditions = map[string]bool{
	"timeout":          true,
	"connection-error": true,
}

const (
	maxAttemptsCap = 1000
	minStatusCode  = 400
	maxStatusCode  = 599
)

type field struct {
	value string
	line  int
}

// Parse reads a policy in the format documented in README.md and validates
// it fully. Every field is checked here so a caller never ends up holding a
// Policy that can't actually be executed.
func Parse(input string) (*Policy, error) {
	fields := map[string]field{}

	for i, raw := range strings.Split(input, "\n") {
		lineNo := i + 1
		line := strings.TrimSpace(strings.TrimRight(raw, "\r"))
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		idx := strings.Index(line, ":")
		if idx < 0 {
			return nil, errf(lineNo, "expected \"key: value\", got %q", line)
		}
		key := strings.TrimSpace(line[:idx])
		value := strings.TrimSpace(line[idx+1:])
		if !allowedKeys[key] {
			return nil, errf(lineNo, "unknown key %q", key)
		}
		if value == "" {
			return nil, errf(lineNo, "%s has no value", key)
		}
		if prev, dup := fields[key]; dup {
			return nil, errf(lineNo, "duplicate key %q (first set on line %d)", key, prev.line)
		}
		fields[key] = field{value: value, line: lineNo}
	}

	maField, ok := fields["max_attempts"]
	if !ok {
		return nil, errf(0, "missing required key \"max_attempts\"")
	}
	maxAttempts, err := parseMaxAttempts(maField)
	if err != nil {
		return nil, err
	}

	boField, ok := fields["backoff"]
	if !ok {
		return nil, errf(0, "missing required key \"backoff\"")
	}
	backoff, err := parseBackoff(boField)
	if err != nil {
		return nil, err
	}

	jitter := JitterNone
	if jf, ok := fields["jitter"]; ok {
		j := Jitter(jf.value)
		if !validJitter[j] {
			return nil, errf(jf.line, "jitter: invalid value %q (want none, full, or equal)", jf.value)
		}
		jitter = j
	}

	var retryOn []string
	if rf, ok := fields["retry_on"]; ok {
		retryOn, err = parseRetryOn(rf)
		if err != nil {
			return nil, err
		}
	}

	return &Policy{
		MaxAttempts: maxAttempts,
		Backoff:     backoff,
		Jitter:      jitter,
		RetryOn:     retryOn,
	}, nil
}

func parseMaxAttempts(f field) (int, error) {
	n, err := strconv.Atoi(f.value)
	if err != nil {
		return 0, errf(f.line, "max_attempts: invalid integer %q", f.value)
	}
	if n < 1 {
		return 0, errf(f.line, "max_attempts: must be at least 1, got %d", n)
	}
	if n > maxAttemptsCap {
		return 0, errf(f.line, "max_attempts: %d exceeds the cap of %d", n, maxAttemptsCap)
	}
	return n, nil
}

func parseDuration(s string, line int, label string) (time.Duration, error) {
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, errf(line, "%s: invalid duration %q", label, s)
	}
	if d <= 0 {
		return 0, errf(line, "%s: must be positive, got %s", label, s)
	}
	return d, nil
}

func parseBackoff(f field) (Backoff, error) {
	parts := strings.Fields(f.value)
	if len(parts) == 0 {
		return Backoff{}, errf(f.line, "backoff: empty value")
	}
	kind := BackoffKind(parts[0])

	params := map[string]string{}
	for _, tok := range parts[1:] {
		eq := strings.Index(tok, "=")
		if eq < 0 {
			return Backoff{}, errf(f.line, "backoff: expected key=value, got %q", tok)
		}
		key := tok[:eq]
		val := tok[eq+1:]
		if val == "" {
			return Backoff{}, errf(f.line, "backoff: %s has no value", key)
		}
		if _, dup := params[key]; dup {
			return Backoff{}, errf(f.line, "backoff: duplicate parameter %q", key)
		}
		params[key] = val
	}

	var required []string
	switch kind {
	case BackoffFixed:
		required = []string{"delay"}
	case BackoffLinear:
		required = []string{"base", "increment", "max"}
	case BackoffExponential:
		required = []string{"base", "max", "multiplier"}
	default:
		return Backoff{}, errf(f.line, "backoff: unknown kind %q (want fixed, linear, or exponential)", parts[0])
	}

	allowed := map[string]bool{}
	for _, k := range required {
		allowed[k] = true
	}
	for k := range params {
		if !allowed[k] {
			return Backoff{}, errf(f.line, "backoff: %q takes no parameter %q", kind, k)
		}
	}
	for _, k := range required {
		if _, ok := params[k]; !ok {
			return Backoff{}, errf(f.line, "backoff: %q requires parameter %q", kind, k)
		}
	}

	b := Backoff{Kind: kind}
	switch kind {
	case BackoffFixed:
		d, err := parseDuration(params["delay"], f.line, "backoff.delay")
		if err != nil {
			return Backoff{}, err
		}
		b.Delay = d

	case BackoffLinear:
		base, err := parseDuration(params["base"], f.line, "backoff.base")
		if err != nil {
			return Backoff{}, err
		}
		inc, err := parseDuration(params["increment"], f.line, "backoff.increment")
		if err != nil {
			return Backoff{}, err
		}
		maxDelay, err := parseDuration(params["max"], f.line, "backoff.max")
		if err != nil {
			return Backoff{}, err
		}
		if base >= maxDelay {
			return Backoff{}, errf(f.line, "backoff: base (%s) must be less than max (%s)", base, maxDelay)
		}
		b.Base, b.Increment, b.Max = base, inc, maxDelay

	case BackoffExponential:
		base, err := parseDuration(params["base"], f.line, "backoff.base")
		if err != nil {
			return Backoff{}, err
		}
		maxDelay, err := parseDuration(params["max"], f.line, "backoff.max")
		if err != nil {
			return Backoff{}, err
		}
		if base >= maxDelay {
			return Backoff{}, errf(f.line, "backoff: base (%s) must be less than max (%s)", base, maxDelay)
		}
		mult, err := strconv.ParseFloat(params["multiplier"], 64)
		if err != nil {
			return Backoff{}, errf(f.line, "backoff.multiplier: invalid number %q", params["multiplier"])
		}
		if mult <= 1.0 {
			return Backoff{}, errf(f.line, "backoff.multiplier: must be greater than 1, got %v", mult)
		}
		b.Base, b.Max, b.Multiplier = base, maxDelay, mult
	}

	return b, nil
}

func parseRetryOn(f field) ([]string, error) {
	raw := strings.Split(f.value, ",")
	seen := map[string]bool{}
	out := make([]string, 0, len(raw))
	for _, tok := range raw {
		tok = strings.TrimSpace(tok)
		if tok == "" {
			return nil, errf(f.line, "retry_on: empty entry")
		}
		if seen[tok] {
			return nil, errf(f.line, "retry_on: duplicate entry %q", tok)
		}
		seen[tok] = true
		out = append(out, tok)
	}

	if len(out) > 1 {
		for _, tok := range out {
			if tok == "any" {
				return nil, errf(f.line, "retry_on: \"any\" can't be combined with other entries")
			}
		}
	}

	for _, tok := range out {
		if tok == "any" || namedConditions[tok] {
			continue
		}
		code, err := strconv.Atoi(tok)
		if err != nil {
			return nil, errf(f.line, "retry_on: invalid entry %q (want a status code, a condition name, or \"any\")", tok)
		}
		if code < minStatusCode || code > maxStatusCode {
			return nil, errf(f.line, "retry_on: status code %d is outside the retryable range %d-%d", code, minStatusCode, maxStatusCode)
		}
	}

	return out, nil
}
