package retrypolicy

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestParseValid(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want *Policy
	}{
		{
			name: "fixed backoff, defaults for jitter and retry_on",
			in: "max_attempts: 3\n" +
				"backoff: fixed delay=500ms\n",
			want: &Policy{
				MaxAttempts: 3,
				Backoff:     Backoff{Kind: BackoffFixed, Delay: 500 * time.Millisecond},
				Jitter:      JitterNone,
			},
		},
		{
			name: "linear backoff with jitter and retry_on",
			in: "max_attempts: 10\n" +
				"backoff: linear base=1s increment=500ms max=10s\n" +
				"jitter: equal\n" +
				"retry_on: 502, 503, timeout\n",
			want: &Policy{
				MaxAttempts: 10,
				Backoff: Backoff{
					Kind:      BackoffLinear,
					Base:      time.Second,
					Increment: 500 * time.Millisecond,
					Max:       10 * time.Second,
				},
				Jitter:  JitterEqual,
				RetryOn: []string{"502", "503", "timeout"},
			},
		},
		{
			name: "exponential backoff with retry_on any",
			in: "max_attempts: 5\n" +
				"backoff: exponential base=200ms max=30s multiplier=2.0\n" +
				"jitter: full\n" +
				"retry_on: any\n",
			want: &Policy{
				MaxAttempts: 5,
				Backoff: Backoff{
					Kind:       BackoffExponential,
					Base:       200 * time.Millisecond,
					Max:        30 * time.Second,
					Multiplier: 2.0,
				},
				Jitter:  JitterFull,
				RetryOn: []string{"any"},
			},
		},
		{
			name: "blank lines and comments are ignored",
			in: "# billing webhook consumer\n" +
				"\n" +
				"max_attempts: 1\n" +
				"\n" +
				"# no point backing off if we only try once\n" +
				"backoff: fixed delay=1s\n",
			want: &Policy{
				MaxAttempts: 1,
				Backoff:     Backoff{Kind: BackoffFixed, Delay: time.Second},
				Jitter:      JitterNone,
			},
		},
		{
			name: "crlf line endings",
			in:   "max_attempts: 2\r\nbackoff: fixed delay=1s\r\n",
			want: &Policy{
				MaxAttempts: 2,
				Backoff:     Backoff{Kind: BackoffFixed, Delay: time.Second},
				Jitter:      JitterNone,
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Parse(tc.in)
			if err != nil {
				t.Fatalf("Parse: unexpected error: %v", err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("Parse() = %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestParseInvalid(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		wantErr string
	}{
		{
			name:    "missing max_attempts",
			in:      "backoff: fixed delay=1s\n",
			wantErr: `missing required key "max_attempts"`,
		},
		{
			name:    "missing backoff",
			in:      "max_attempts: 3\n",
			wantErr: `missing required key "backoff"`,
		},
		{
			name: "duplicate key",
			in: "max_attempts: 3\n" +
				"max_attempts: 5\n" +
				"backoff: fixed delay=1s\n",
			wantErr: `duplicate key "max_attempts"`,
		},
		{
			name: "unknown key",
			in: "max_attempts: 3\n" +
				"backoff: fixed delay=1s\n" +
				"timeout: 5s\n",
			wantErr: `unknown key "timeout"`,
		},
		{
			name: "empty value",
			in: "max_attempts: 3\n" +
				"backoff: fixed delay=1s\n" +
				"jitter:\n",
			wantErr: "jitter has no value",
		},
		{
			name: "line without a colon",
			in: "max_attempts 3\n" +
				"backoff: fixed delay=1s\n",
			wantErr: "expected \"key: value\"",
		},
		{
			name: "max_attempts not an integer",
			in: "max_attempts: five\n" +
				"backoff: fixed delay=1s\n",
			wantErr: "max_attempts: invalid integer",
		},
		{
			name: "max_attempts zero",
			in: "max_attempts: 0\n" +
				"backoff: fixed delay=1s\n",
			wantErr: "must be at least 1",
		},
		{
			name: "max_attempts exceeds cap",
			in: "max_attempts: 1001\n" +
				"backoff: fixed delay=1s\n",
			wantErr: "exceeds the cap",
		},
		{
			name: "unknown backoff kind",
			in: "max_attempts: 3\n" +
				"backoff: quadratic delay=1s\n",
			wantErr: "unknown kind",
		},
		{
			name: "backoff kind is case sensitive",
			in: "max_attempts: 3\n" +
				"backoff: Fixed delay=1s\n",
			wantErr: "unknown kind",
		},
		{
			name: "backoff missing required parameter",
			in: "max_attempts: 3\n" +
				"backoff: fixed\n",
			wantErr: `requires parameter "delay"`,
		},
		{
			name: "backoff unknown parameter for kind",
			in: "max_attempts: 3\n" +
				"backoff: fixed delay=1s jitter_seed=42\n",
			wantErr: `takes no parameter "jitter_seed"`,
		},
		{
			name: "backoff duplicate parameter",
			in: "max_attempts: 3\n" +
				"backoff: fixed delay=1s delay=2s\n",
			wantErr: `duplicate parameter "delay"`,
		},
		{
			name: "backoff parameter missing equals sign",
			in: "max_attempts: 3\n" +
				"backoff: fixed 1s\n",
			wantErr: "expected key=value",
		},
		{
			name: "duration with invalid unit",
			in: "max_attempts: 3\n" +
				"backoff: fixed delay=500xyz\n",
			wantErr: "invalid duration",
		},
		{
			name: "duration not positive",
			in: "max_attempts: 3\n" +
				"backoff: fixed delay=0s\n",
			wantErr: "must be positive",
		},
		{
			name: "linear base not less than max",
			in: "max_attempts: 3\n" +
				"backoff: linear base=30s increment=1s max=10s\n",
			wantErr: "must be less than max",
		},
		{
			name: "exponential multiplier not greater than one",
			in: "max_attempts: 3\n" +
				"backoff: exponential base=1s max=30s multiplier=1\n",
			wantErr: "must be greater than 1",
		},
		{
			name: "exponential multiplier not a number",
			in: "max_attempts: 3\n" +
				"backoff: exponential base=1s max=30s multiplier=fast\n",
			wantErr: "invalid number",
		},
		{
			name: "jitter invalid value",
			in: "max_attempts: 3\n" +
				"backoff: fixed delay=1s\n" +
				"jitter: sometimes\n",
			wantErr: "invalid value",
		},
		{
			name: "jitter is case sensitive",
			in: "max_attempts: 3\n" +
				"backoff: fixed delay=1s\n" +
				"jitter: Full\n",
			wantErr: "invalid value",
		},
		{
			name: "retry_on trailing comma leaves an empty entry",
			in: "max_attempts: 3\n" +
				"backoff: fixed delay=1s\n" +
				"retry_on: 502, 503,\n",
			wantErr: "empty entry",
		},
		{
			name: "retry_on duplicate entry",
			in: "max_attempts: 3\n" +
				"backoff: fixed delay=1s\n" +
				"retry_on: 502, 502\n",
			wantErr: `duplicate entry "502"`,
		},
		{
			name: "retry_on any combined with other entries",
			in: "max_attempts: 3\n" +
				"backoff: fixed delay=1s\n" +
				"retry_on: any, 502\n",
			wantErr: `"any" can't be combined`,
		},
		{
			name: "retry_on status code out of range",
			in: "max_attempts: 3\n" +
				"backoff: fixed delay=1s\n" +
				"retry_on: 200\n",
			wantErr: "outside the retryable range",
		},
		{
			name: "retry_on unrecognized token",
			in: "max_attempts: 3\n" +
				"backoff: fixed delay=1s\n" +
				"retry_on: bogus\n",
			wantErr: "invalid entry",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Parse(tc.in)
			if err == nil {
				t.Fatalf("Parse: expected an error containing %q, got nil", tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("Parse: error %q does not contain %q", err.Error(), tc.wantErr)
			}
		})
	}
}

func TestRoundTrip(t *testing.T) {
	policies := []*Policy{
		{
			MaxAttempts: 1,
			Backoff:     Backoff{Kind: BackoffFixed, Delay: 250 * time.Millisecond},
			Jitter:      JitterNone,
		},
		{
			MaxAttempts: 8,
			Backoff: Backoff{
				Kind:      BackoffLinear,
				Base:      time.Second,
				Increment: 2 * time.Second,
				Max:       20 * time.Second,
			},
			Jitter:  JitterEqual,
			RetryOn: []string{"502", "503", "504"},
		},
		{
			MaxAttempts: 6,
			Backoff: Backoff{
				Kind:       BackoffExponential,
				Base:       100 * time.Millisecond,
				Max:        1 * time.Minute,
				Multiplier: 1.5,
			},
			Jitter:  JitterFull,
			RetryOn: []string{"any"},
		},
	}

	for _, want := range policies {
		got, err := Parse(want.String())
		if err != nil {
			t.Fatalf("Parse(%q): unexpected error: %v", want.String(), err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("round trip mismatch:\nprinted: %s\ngot:  %+v\nwant: %+v", want.String(), got, want)
		}
	}
}
