package backoff

import (
	"testing"
	"time"
)

func TestGetDelay(t *testing.T) {
	type args struct {
		s Strategy
	}
	tests := []struct {
		name string
		args args
		want time.Duration
	}{
		{
			name: "default - no delay on first attempt",
			args: args{
				s: Strategy{
					Type:             Default,
					Interval:         5,
					PreviousAttempts: 0,
				},
			},
			want: 0 * time.Second,
		},
		{
			name: "default - 5s delay on second attempt",
			args: args{
				s: Strategy{
					Type:             Default,
					Interval:         5,
					PreviousAttempts: 1,
				},
			},
			want: 5 * time.Second,
		},
		{
			name: "default - 5s delay on fifth attempt",
			args: args{
				s: Strategy{
					Type:             Default,
					Interval:         5,
					PreviousAttempts: 5,
				},
			},
			want: 5 * time.Second,
		},
		{
			name: "default - 5s delay on tenth attempt",
			args: args{
				s: Strategy{
					Type:             Default,
					Interval:         5,
					PreviousAttempts: 10,
				},
			},
			want: 5 * time.Second,
		},

		// EXP - Interval of 1
		{
			name: "exponential - interval of 1 - no delay on no previous attempts",
			args: args{
				s: Strategy{
					Type:             Exponential,
					Interval:         1,
					PreviousAttempts: 0,
				},
			},
			want: 0 * time.Second,
		},
		{
			name: "exponential - interval of 1 - no delay on one previous attempt",
			args: args{
				s: Strategy{
					Type:             Exponential,
					Interval:         1,
					PreviousAttempts: 1,
				},
			},
			want: 0 * time.Second,
		},
		{
			name: "exponential - interval of 1 - 1s delay on two previous attempts",
			args: args{
				s: Strategy{
					Type:             Exponential,
					Interval:         1,
					PreviousAttempts: 2,
				},
			},
			want: 1 * time.Second,
		},
		{
			name: "exponential - interval of 1 - 3s delay on three previous attempts",
			args: args{
				s: Strategy{
					Type:             Exponential,
					Interval:         1,
					PreviousAttempts: 3,
				},
			},
			want: 3 * time.Second,
		},
		{
			name: "exponential - interval of 1 - 7s delay on four previous attempts",
			args: args{
				s: Strategy{
					Type:             Exponential,
					Interval:         1,
					PreviousAttempts: 4,
				},
			},
			want: 7 * time.Second,
		},

		// EXP - Interval of 3
		{
			name: "exponential - interval of 3 - no delay on no previous attempts",
			args: args{
				s: Strategy{
					Type:             Exponential,
					Interval:         3,
					PreviousAttempts: 0,
				},
			},
			want: 0 * time.Second,
		},
		{
			name: "exponential - interval of 3 - 3s delay on one previous attempt",
			args: args{
				s: Strategy{
					Type:             Exponential,
					Interval:         3,
					PreviousAttempts: 1,
				},
			},
			want: 3 * time.Second,
		},
		{
			name: "exponential - interval of 3 - 31s delay on two previous attempts",
			args: args{
				s: Strategy{
					Type:             Exponential,
					Interval:         3,
					PreviousAttempts: 2,
				},
			},
			want: 31 * time.Second,
		},
		{
			name: "exponential - interval of 3 - 255s delay on three previous attempts",
			args: args{
				s: Strategy{
					Type:             Exponential,
					Interval:         3,
					PreviousAttempts: 3,
				},
			},
			want: 255 * time.Second,
		},
		{
			name: "exponential - interval of 3 - 2047s delay on four previous attempts",
			args: args{
				s: Strategy{
					Type:             Exponential,
					Interval:         3,
					PreviousAttempts: 4,
				},
			},
			want: 2047 * time.Second,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetDelay(tt.args.s); got != tt.want {
				t.Errorf("GetDelay() = %v, want %v", got, tt.want)
			}
		})
	}
}
