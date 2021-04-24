package conman

import (
	"fmt"
	"testing"
	"time"
)

type doubler struct {
	operand int
}

func (d *doubler) Execute() (interface{}, error) {
	return d.operand * 2, nil
}

type errdoubler struct {
	operand int
}

func (d *errdoubler) Execute() (interface{}, error) {
	return nil, fmt.Errorf("Error calculating for %v", d.operand)
}

type slowdoubler struct {
	operand int
}

func (d *slowdoubler) Execute() (interface{}, error) {
	time.Sleep(time.Second)
	return d.operand * 2, nil
}

func TestCaptureOutputs(t *testing.T) {
	cm := New(5)

	cm.Run(&doubler{operand: 299})
	cm.Run(&doubler{operand: 532})
	cm.Run(&doubler{operand: 203})

	cm.Wait()

	for _, o := range []int{598, 1064, 406} {
		if !contains(cm.Outputs(), o) {
			t.Errorf("Expected output %v is not part of the captured outputs", o)
		}
	}
}

func TestCaptureErrors(t *testing.T) {
	cm := New(5)

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
	cm := New(2)

	cm.Run(&slowdoubler{operand: 299})
	cm.Run(&slowdoubler{operand: 532})
	cm.Run(&slowdoubler{operand: 203})

	// Wait to make sure the first two tasks have completed
	time.Sleep(100 * time.Millisecond)
	if contains(cm.Outputs(), 406) {
		t.Errorf("Didn't expect task for 406 to have been executed")
	}
	cm.Wait()

	if !contains(cm.Outputs(), 406) {
		t.Errorf("Expected task for 406 to have been exectued")
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

func contains(items []interface{}, item interface{}) bool {
	for _, i := range items {
		if i == item {
			return true
		}
	}
	return false
}
