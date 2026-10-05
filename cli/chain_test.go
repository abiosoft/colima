package cli

import (
	"context"
	"fmt"
	"reflect"
	"testing"
	"time"
)

// quietChain returns an ActiveCommandChain whose logger discards output, keeping tests silent.
func quietChain() *ActiveCommandChain {
	ctx := context.WithValue(context.Background(), CtxKeyQuiet, true)
	return New("test").Init(ctx)
}

func TestRetry(t *testing.T) {
	tests := []struct {
		name string
		// count is the requested number of attempts.
		count int
		// fail reports whether f should error for the given retryCount (1-based).
		// Returning false makes f succeed, stopping further attempts.
		fail func(retryCount int) bool
		// wantCalls is the exact number of times f must be invoked.
		wantCalls int
		// wantRetryCount is the exact 1-based retryCount sequence f must observe.
		wantRetryCount []int
		// wantErr is "" when Exec must return nil, otherwise the expected last error message.
		wantErr string
	}{
		{
			name:           "all attempts fail",
			count:          3,
			fail:           func(int) bool { return true },
			wantCalls:      3,
			wantRetryCount: []int{1, 2, 3},
			wantErr:        "failed at retryCount 3",
		},
		{
			name:           "single attempt fails",
			count:          1,
			fail:           func(int) bool { return true },
			wantCalls:      1,
			wantRetryCount: []int{1},
			wantErr:        "failed at retryCount 1",
		},
		{
			name:           "succeeds on first attempt",
			count:          4,
			fail:           func(int) bool { return false },
			wantCalls:      1,
			wantRetryCount: []int{1},
			wantErr:        "",
		},
		{
			name:           "succeeds on second attempt",
			count:          5,
			fail:           func(retryCount int) bool { return retryCount < 2 },
			wantCalls:      2,
			wantRetryCount: []int{1, 2},
			wantErr:        "",
		},
		{
			name:           "succeeds on final attempt",
			count:          3,
			fail:           func(retryCount int) bool { return retryCount < 3 },
			wantCalls:      3,
			wantRetryCount: []int{1, 2, 3},
			wantErr:        "",
		},
		{
			name:           "zero attempts when count is zero",
			count:          0,
			fail:           func(int) bool { return true },
			wantCalls:      0,
			wantRetryCount: nil,
			wantErr:        "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var (
				calls      int
				retryCount []int
			)

			active := quietChain()
			active.Retry("", time.Millisecond, tt.count, func(rc int) error {
				calls++
				retryCount = append(retryCount, rc)
				if tt.fail(rc) {
					return fmt.Errorf("failed at retryCount %d", rc)
				}
				return nil
			})

			err := active.Exec()

			if calls != tt.wantCalls {
				t.Errorf("f invoked %d time(s), want %d", calls, tt.wantCalls)
			}
			if !reflect.DeepEqual(retryCount, tt.wantRetryCount) {
				t.Errorf("retryCount sequence = %v, want %v", retryCount, tt.wantRetryCount)
			}
			gotErr := ""
			if err != nil {
				gotErr = err.Error()
			}
			if gotErr != tt.wantErr {
				t.Errorf("Exec() error = %q, want %q", gotErr, tt.wantErr)
			}
		})
	}
}
