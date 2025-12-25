// Author: Ilyess Bachiri
// Copyright (c) 2021-present Ilyess Bachiri

package conman

import (
	"context"
	"fmt"
	"slices"
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
	delayInSeconds int
	operand        int
}

func (d *slowdoubler) Execute(ctx context.Context) (int, error) {
	delay := d.delayInSeconds
	if delay == 0 {
		delay = 1 // default to 1 second if not specified
	}
	time.Sleep(time.Duration(delay) * time.Second)
	select {
	case <-ctx.Done():
		return -1, ctx.Err()
	default:
		return d.operand * 2, nil
	}
}

func TestCaptureOutputs(t *testing.T) {
	ctx := t.Context()
	cm := New[int](5)

	cm.Run(ctx, &doubler{operand: 299})
	cm.Run(ctx, &doubler{operand: 532})
	cm.Run(ctx, &doubler{operand: 203})

	cm.Wait()

	for _, o := range []int{598, 1064, 406} {
		if !slices.Contains(cm.Outputs(), o) {
			t.Errorf("Expected output %v is not part of the captured outputs", o)
		}
	}
}

func TestCaptureErrors(t *testing.T) {
	ctx := t.Context()
	cm := New[int](5)

	cm.Run(ctx, &errdoubler{operand: 299})
	cm.Run(ctx, &errdoubler{operand: 532})
	cm.Run(ctx, &errdoubler{operand: 203})

	cm.Wait()

	for _, i := range []int{299, 532, 203} {
		if !containsError(cm.Errors(), fmt.Errorf("Error calculating for %v", i)) {
			t.Errorf("Expected error for %v but none was found", i)
		}
	}
}

func TestConcurrencyLimit(t *testing.T) {
	ctx := t.Context()
	cm := New[int](2)

	cm.Run(ctx, &slowdoubler{operand: 299})
	cm.Run(ctx, &slowdoubler{operand: 532})
	cm.Run(ctx, &slowdoubler{operand: 203})

	// Wait to make sure the first two tasks have completed
	time.Sleep(100 * time.Millisecond)
	if slices.Contains(cm.Outputs(), 406) {
		t.Errorf("Didn't expect task for 406 to have been executed")
	}
	cm.Wait()

	if !slices.Contains(cm.Outputs(), 406) {
		t.Errorf("Expected task for 406 to have been exectued")
	}
}

func TestRetries(t *testing.T) {
	ctx := t.Context()
	cm := New[int](3)

	cm.Run(ctx, &flakydoubler{operand: 299})
	cm.Run(ctx, &flakydoubler{operand: 532})
	cm.Run(ctx, &flakydoubler{operand: 203})

	cm.Wait()
	for _, o := range []int{598, 1064, 406} {
		if !slices.Contains(cm.Outputs(), o) {
			t.Errorf("Expected output %v is not part of the captured outputs", o)
		}
	}
}

func TestMaxRetries(t *testing.T) {
	ctx := t.Context()
	cm := New[int](3)

	cm.Run(ctx, &faultydoubler{operand: 299})
	cm.Run(ctx, &faultydoubler{operand: 532})
	cm.Run(ctx, &faultydoubler{operand: 203})

	cm.Wait()
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
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	cm := New[int](3)

	cm.Run(ctx, &slowdoubler{operand: 299})
	time.Sleep(4 * time.Second)
	if err := cm.Run(ctx, &slowdoubler{operand: 532}); err == nil {
		t.Errorf("Expected context deadline exceeded error but got nil")
	}
	if err := cm.Run(ctx, &slowdoubler{operand: 203}); err == nil {
		t.Errorf("Expected context deadline exceeded error but got nil")
	}

	cm.Wait()
	if !slices.Contains(cm.Outputs(), 598) {
		t.Errorf("Expected output %v is not part of the captured outputs", 598)
	}
}

func TestContextPropagation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	cm := New[int](3)
	cm.Run(ctx, &slowdoubler{operand: 299, delayInSeconds: 1})
	cm.Run(ctx, &slowdoubler{operand: 532, delayInSeconds: 2})
	cm.Run(ctx, &slowdoubler{operand: 203, delayInSeconds: 6})

	go func() {
		<-time.After(5 * time.Second)
		cancel()
	}()

	cm.Wait()
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
	err := cm.Errors()[0].Error()
	if err != "context canceled" {
		t.Errorf("Expected a 'context canceled' error, got '%s'", err)
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
