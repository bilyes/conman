// Author: Ilyess Bachiri
// Copyright (c) 2021-present Ilyess Bachiri

package conman

import (
	"fmt"
	"slices"
	"testing"
	"time"
)

type flakydoubler struct {
	operand  int
	runCount int
}

func (f *flakydoubler) Execute() (int, error) {
	if f.runCount < 2 {
		f.runCount++
		return -1, &RetriableError{Err: fmt.Errorf("Try again"), MaxRetries: 2}
	}

	return f.operand * 2, nil
}

type faultydoubler struct {
	operand int
}

func (f *faultydoubler) Execute() (int, error) {
	return -1, &RetriableError{Err: fmt.Errorf("Try again"), MaxRetries: 2}
}

type doubler struct {
	operand int
}

func (d *doubler) Execute() (int, error) {
	return d.operand * 2, nil
}

type errdoubler struct {
	operand int
}

func (d *errdoubler) Execute() (int, error) {
	return -1, fmt.Errorf("Error calculating for %v", d.operand)
}

type slowdoubler struct {
	operand int
}

func (d *slowdoubler) Execute() (int, error) {
	time.Sleep(time.Second)
	return d.operand * 2, nil
}

func TestCaptureOutputs(t *testing.T) {
	cm := New[int](5)

	cm.Run(&doubler{operand: 299})
	cm.Run(&doubler{operand: 532})
	cm.Run(&doubler{operand: 203})

	cm.Wait()

	for _, o := range []int{598, 1064, 406} {
		if !slices.Contains(cm.Outputs(), o) {
			t.Errorf("Expected output %v is not part of the captured outputs", o)
		}
	}
}

func TestCaptureErrors(t *testing.T) {
	cm := New[int](5)

	cm.Run(&errdoubler{operand: 299})
	cm.Run(&errdoubler{operand: 532})
	cm.Run(&errdoubler{operand: 203})

	cm.Wait()

	for _, i := range []int{299, 532, 203} {
		if !containsError(cm.Errors(), fmt.Errorf("Error calculating for %v", i)) {
			t.Errorf("Expected error for %v but none was found", i)
		}
	}
}

func TestConcurrencyLimit(t *testing.T) {
	cm := New[int](2)

	cm.Run(&slowdoubler{operand: 299})
	cm.Run(&slowdoubler{operand: 532})
	cm.Run(&slowdoubler{operand: 203})

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
	cm := New[int](3)

	cm.Run(&flakydoubler{operand: 299})
	cm.Run(&flakydoubler{operand: 532})
	cm.Run(&flakydoubler{operand: 203})

	cm.Wait()
	for _, o := range []int{598, 1064, 406} {
		if !slices.Contains(cm.Outputs(), o) {
			t.Errorf("Expected output %v is not part of the captured outputs", o)
		}
	}
}

func TestMaxRetries(t *testing.T) {
	cm := New[int](3)

	cm.Run(&faultydoubler{operand: 299})
	cm.Run(&faultydoubler{operand: 532})
	cm.Run(&faultydoubler{operand: 203})

	cm.Wait()
	for _, o := range []int{598, 1064, 406} {
		if slices.Contains(cm.Outputs(), o) {
			t.Errorf("Didn't expect output %v in the captured outputs", o)
		}
	}

	errCount := len(cm.Errors())
	if errCount != 3 {
		t.Errorf("Expected 3 errors, got %d", errCount)
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
