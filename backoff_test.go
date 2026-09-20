package retrypolicy

import (
	"reflect"
	"testing"
	"time"
)

func TestBackoffDelays(t *testing.T) {
	cases := []struct {
		name     string
		b        Backoff
		attempts int
		want     []time.Duration
	}{
		{
			name:     "fixed",
			b:        Backoff{Kind: BackoffFixed, Delay: 2 * time.Second},
			attempts: 4,
			want:     []time.Duration{2 * time.Second, 2 * time.Second, 2 * time.Second},
		},
		{
			name: "linear grows then caps",
			b: Backoff{
				Kind:      BackoffLinear,
				Base:      time.Second,
				Increment: 500 * time.Millisecond,
				Max:       2500 * time.Millisecond,
			},
			attempts: 6,
			want: []time.Duration{
				time.Second,
				1500 * time.Millisecond,
				2 * time.Second,
				2500 * time.Millisecond,
				2500 * time.Millisecond,
			},
		},
		{
			name: "exponential grows then caps",
			b: Backoff{
				Kind:       BackoffExponential,
				Base:       200 * time.Millisecond,
				Max:        1600 * time.Millisecond,
				Multiplier: 2,
			},
			attempts: 6,
			want: []time.Duration{
				200 * time.Millisecond,
				400 * time.Millisecond,
				800 * time.Millisecond,
				1600 * time.Millisecond,
				1600 * time.Millisecond,
			},
		},
		{
			name:     "one attempt needs no delays",
			b:        Backoff{Kind: BackoffFixed, Delay: time.Second},
			attempts: 1,
			want:     nil,
		},
		{
			name:     "zero or negative attempts needs no delays",
			b:        Backoff{Kind: BackoffFixed, Delay: time.Second},
			attempts: 0,
			want:     nil,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.b.Delays(tc.attempts)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("Delays(%d) = %v, want %v", tc.attempts, got, tc.want)
			}
		})
	}
}
