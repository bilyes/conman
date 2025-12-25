// Author: Ilyess Bachiri
// Copyright (c) 2021-present Ilyess Bachiri

package conman

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"
)

type flakydoubler struct {
	operand  int
	runCount int
}

func (f *flakydoubler) Execute(ctx context.Context) (int, error) {
	if f.runCount < 2 {
		f.runCount++
		err := &RetriableError{Err: fmt.Errorf("Try again")}
		return -1, err.WithExponentialBackoff()
	}

	return f.operand * 2, nil
}

type faultydoubler struct {
	operand int
}

func (f *faultydoubler) Execute(ctx context.Context) (int, error) {
	err := &RetriableError{Err: fmt.Errorf("Try again")}
	return -1, err.WithLinearBackoff()
}

type doubler struct {
	operand int
}

func (d *doubler) Execute(ctx context.Context) (int, error) {
	return d.operand * 2, nil
}

type errdoubler struct {
	operand int
}

func (d *errdoubler) Execute(ctx context.Context) (int, error) {
	return -1, fmt.Errorf("Error calculating for %v", d.operand)
}

type slowdoubler struct {
	delayInMiliseconds int
	operand            int
}

func (d *slowdoubler) Execute(ctx context.Context) (int, error) {
	delay := d.delayInMiliseconds
	if delay == 0 {
		delay = 100 // default to 100 ms if not specified
	}
	time.Sleep(time.Duration(delay) * time.Millisecond)
	select {
	case <-ctx.Done():
		return -1, ctx.Err()
	default:
		return d.operand * 2, nil
	}
}

func TestCaptureOutputs(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	cm, err := New[int](5)
	if err != nil {
		t.Fatalf("Failed to create ConMan: %v", err)
	}

	cm.Run(ctx, &doubler{operand: 299})
	cm.Run(ctx, &doubler{operand: 532})
	cm.Run(ctx, &doubler{operand: 203})

	if err := cm.Wait(ctx); err != nil {
		t.Fatalf("ConMan Wait returned an unexpected error: %v", err)
	}

	for _, o := range []int{598, 1064, 406} {
		if !slices.Contains(cm.Outputs(), o) {
			t.Errorf("Expected output %v is not part of the captured outputs", o)
		}
	}
}

func TestCaptureErrors(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	cm, err := New[int](5)
	if err != nil {
		t.Fatalf("Failed to create ConMan: %v", err)
	}

	cm.Run(ctx, &errdoubler{operand: 299})
	cm.Run(ctx, &errdoubler{operand: 532})
	cm.Run(ctx, &errdoubler{operand: 203})

	if err := cm.Wait(ctx); err != nil {
		t.Fatalf("ConMan Wait returned an unexpected error: %v", err)
	}

	for _, i := range []int{299, 532, 203} {
		if !containsError(cm.Errors(), fmt.Errorf("Error calculating for %v", i)) {
			t.Errorf("Expected error for %v but none was found", i)
		}
	}
}

func TestConcurrencyLimit(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	cm, err := New[int](2)
	if err != nil {
		t.Fatalf("Failed to create ConMan: %v", err)
	}

	cm.Run(ctx, &slowdoubler{operand: 299})
	cm.Run(ctx, &slowdoubler{operand: 532})
	cm.Run(ctx, &slowdoubler{operand: 203})

	// Wait to make sure the first two tasks have completed
	time.Sleep(10 * time.Millisecond)
	if slices.Contains(cm.Outputs(), 406) {
		t.Errorf("Didn't expect task for 406 to have been executed")
	}
	if err := cm.Wait(ctx); err != nil {
		t.Fatalf("ConMan Wait returned an unexpected error: %v", err)
	}

	if !slices.Contains(cm.Outputs(), 406) {
		t.Errorf("Expected task for 406 to have been exectued")
	}
}

func TestRetries(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	cm, err := New[int](3)
	if err != nil {
		t.Fatalf("Failed to create ConMan: %v", err)
	}

	cm.Run(ctx, &flakydoubler{operand: 299})
	cm.Run(ctx, &flakydoubler{operand: 532})
	cm.Run(ctx, &flakydoubler{operand: 203})

	if err := cm.Wait(ctx); err != nil {
		t.Fatalf("ConMan Wait returned an unexpected error: %v", err)
	}
	for _, o := range []int{598, 1064, 406} {
		if !slices.Contains(cm.Outputs(), o) {
			t.Errorf("Expected output %v is not part of the captured outputs", o)
		}
	}
}

func TestMaxRetries(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	cm, err := New[int](3)
	if err != nil {
		t.Fatalf("Failed to create ConMan: %v", err)
	}

	cm.Run(ctx, &faultydoubler{operand: 299})
	cm.Run(ctx, &faultydoubler{operand: 532})
	cm.Run(ctx, &faultydoubler{operand: 203})

	if err := cm.Wait(ctx); err != nil {
		t.Fatalf("ConMan Wait returned an unexpected error: %v", err)
	}
	for _, o := range []int{598, 1064, 406} {
		if slices.Contains(cm.Outputs(), o) {
			t.Errorf("Didn't expect output %v in the captured outputs", o)
		}
	}

	if errCount := len(cm.Errors()); errCount != 3 {
		t.Errorf("Expected 3 errors, got %d", errCount)
	}
}

func TestDispatchTimeout(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	cm, err := New[int](3)
	if err != nil {
		t.Fatalf("Failed to create ConMan: %v", err)
	}

	cm.Run(ctx, &slowdoubler{operand: 299})
	time.Sleep(300 * time.Millisecond) // Ensure the context times out before next runs
	if err := cm.Run(ctx, &slowdoubler{operand: 532}); err == nil {
		t.Errorf("Expected context deadline exceeded error but got nil")
	}
	if err := cm.Run(ctx, &slowdoubler{operand: 203}); err == nil {
		t.Errorf("Expected context deadline exceeded error but got nil")
	}

	if err := cm.Wait(context.Background()); err != nil {
		t.Fatalf("ConMan Wait returned an unexpected error: %v", err)
	}
	if !slices.Contains(cm.Outputs(), 598) {
		t.Errorf("Expected output %v is not part of the captured outputs", 598)
	}
}

func TestContextPropagation(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())

	cm, err := New[int](3)
	if err != nil {
		t.Fatalf("Failed to create ConMan: %v", err)
	}
	cm.Run(ctx, &slowdoubler{operand: 299, delayInMiliseconds: 100})
	cm.Run(ctx, &slowdoubler{operand: 532, delayInMiliseconds: 200})
	cm.Run(ctx, &slowdoubler{operand: 203, delayInMiliseconds: 400})

	go func() {
		<-time.After(300 * time.Millisecond)
		cancel()
	}()

	if err := cm.Wait(context.Background()); err != nil {
		t.Fatalf("ConMan Wait returned an unexpected error: %v", err)
	}
	for _, o := range []int{598, 1064} {
		if !slices.Contains(cm.Outputs(), o) {
			t.Errorf("Expected output %v is not part of the captured outputs", o)
		}
	}

	if slices.Contains(cm.Outputs(), 406) {
		t.Errorf("Didn't expect output %v in the captured outputs", 406)
	}
	if len(cm.Errors()) < 1 {
		t.Error("Expected one error, got none")
	}
	errMsg := cm.Errors()[0].Error()
	if errMsg != "context canceled" {
		t.Errorf("Expected a 'context canceled' error, got '%s'", errMsg)
	}
}

func TestNewValidation(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name             string
		concurrencyLimit int64
		expectError      bool
		errorContains    string
	}{
		{
			name:             "Valid concurrency limit 2",
			concurrencyLimit: 2,
			expectError:      false,
		},
		{
			name:             "Valid concurrency limit 5",
			concurrencyLimit: 5,
			expectError:      false,
		},
		{
			name:             "Invalid concurrency limit 1",
			concurrencyLimit: 1,
			expectError:      true,
			errorContains:    "concurrencyLimit must be at least 2, got 1",
		},
		{
			name:             "Invalid concurrency limit 0",
			concurrencyLimit: 0,
			expectError:      true,
			errorContains:    "concurrencyLimit must be at least 2, got 0",
		},
		{
			name:             "Invalid concurrency limit negative",
			concurrencyLimit: -5,
			expectError:      true,
			errorContains:    "concurrencyLimit must be at least 2, got -5",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cm, err := New[int](tt.concurrencyLimit)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
					return
				}
				if tt.errorContains != "" && !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("Expected error to contain '%s', got '%s'", tt.errorContains, err.Error())
				}
				if cm != nil {
					t.Errorf("Expected nil ConMan on error, got %v", cm)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
					return
				}
				if cm == nil {
					t.Errorf("Expected non-nil ConMan, got nil")
				}
			}
		})
	}
}

func containsError(items []error, item error) bool {
	for _, i := range items {
		if i.Error() == item.Error() {
			return true
		}
	}
	return false
}
