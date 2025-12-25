// Author: Ilyess Bachiri
// Copyright (c) 2021-present Ilyess Bachiri

// Package conman provides a concurrency manager that allows setting a limit to the number
// of tasks that can run concurrently. It provides an intuitive interface for defining
// and concurrently running any type of tasks.
//
// Basic usage:
//
//	cm, err := conman.New[int](5) // concurrency limit of 5 (minimum is 2)
//	if err != nil {
//		log.Fatal(err)
//	}
//
//	// Define a task
//	type myTask struct{}
//	func (t *myTask) Execute(ctx context.Context) (int, error) {
//		return 42, nil
//	}
//
//	// Run the task
//	err = cm.Run(ctx, &myTask{})
//
//	// Wait for completion and get results
//	cm.Wait()
//	results := cm.Outputs()
//
// The package also supports automatic retry mechanisms with configurable backoff strategies
// for tasks that may fail temporarily.
package conman

import (
	"context"
	"fmt"
	"math"
	"math/rand/v2"
	"sync"
	"time"
)

// ConMan a structure to manage multiple tasks running
// concurrently while ensuring the total number of running
// tasks doesn't exceed a certain concurrency limit
type ConMan[T any] struct {
	wg      sync.WaitGroup
	mu      sync.Mutex
	errors  []error
	outputs []T
	buffer  chan any
}

// New creates a new ConMan instance with the specified concurrency limit.
//
// The concurrency limit determines the maximum number of tasks that can run concurrently.
// The limit must be at least 2 to ensure meaningful concurrency.
//
// Parameters:
//   - concurrencyLimit: Maximum number of concurrent tasks (must be ≥ 2)
//
// Returns:
//   - *ConMan[T]: A new ConMan instance
//   - error: An error if concurrencyLimit is less than 2
//
// Example:
//
//	cm, err := conman.New[int](5) // Allow up to 5 concurrent tasks
//	if err != nil {
//		return fmt.Errorf("failed to create ConMan: %w", err)
//	}
func New[T any](concurrencyLimit int64) (*ConMan[T], error) {
	if concurrencyLimit < 2 {
		return nil, fmt.Errorf("concurrencyLimit must be at least 2, got %d", concurrencyLimit)
	}
	return &ConMan[T]{
		buffer:  make(chan any, concurrencyLimit),
		outputs: make([]T, 0, concurrencyLimit), // Preallocate for all tasks
		errors:  make([]error, 0),               // Let errors grow as needed (typically fewer)
	}, nil
}

// Task defines the interface that all executable tasks must implement.
// Any type that implements this method can be run concurrently through the ConMan.
//
// The Execute method should be context-aware and respect context cancellation.
// It can optionally return a *RetriableError to trigger automatic retry logic.
type Task[T any] interface {
	// Execute runs the task with the provided context.
	//
	// Parameters:
	//   - ctx: Context for cancellation and timeout control
	//
	// Returns:
	//   - T: The result of the task execution
	//   - error: Any error encountered during execution
	//            Return *RetriableError to trigger retry logic
	Execute(ctx context.Context) (T, error)
}

// Run executes a task concurrently, respecting the concurrency limit.
//
// If the concurrency limit is reached, this method blocks until a slot becomes available.
// The task runs in a separate goroutine and results are collected automatically.
//
// Parameters:
//   - ctx: Context for cancellation and timeout control
//   - t: Task implementing the Task[T] interface
//
// Returns:
//   - error: Context cancellation error if ctx is cancelled before task starts
//     Returns nil if task is successfully dispatched
//
// Note: This method only returns errors related to task dispatch.
//
//	Task execution errors are collected and accessible via Errors().
func (c *ConMan[T]) Run(ctx context.Context, t Task[T]) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		c.reserveOne()
		go func() {
			defer c.releaseOne()
			c.executeTask(ctx, t)
		}()
	}
	return nil
}

// Wait blocks until all previously dispatched tasks have completed.
//
// This method should be called after all Run() calls to ensure all tasks
// have finished execution before accessing results or errors.
//
// Parameters:
//   - ctx: Context for cancellation and timeout control
//
// Returns:
//   - error: Context cancellation error if ctx is cancelled before all tasks complete
//     Returns nil if all tasks complete successfully
//
// Note: This method blocks until all tasks finish or context is cancelled.
// After calling Wait(), you can access results via Outputs() and errors via Errors().
func (c *ConMan[T]) Wait(ctx context.Context) error {
	done := make(chan struct{})

	go func() {
		c.wg.Wait()
		close(done)
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-done:
		return nil
	}
}

// Outputs returns a slice of successful task results.
//
// Only results from tasks that completed without errors are included.
// Results are collected in the order tasks complete, not submission order.
//
// Returns:
//   - []T: Slice of successful task results
func (c *ConMan[T]) Outputs() []T {
	var result []T
	c.withLock(func() {
		result = c.outputs
	})
	return result
}

// Errors returns a slice of all task execution errors.
//
// Only errors from tasks that failed during execution are included.
// Errors are collected in the order they occur.
//
// Returns:
//   - []error: Slice of task execution errors
func (c *ConMan[T]) Errors() []error {
	var result []error
	c.withLock(func() {
		result = c.errors
	})
	return result
}

// reserveOne reserves a slot in the concurrency buffer and increments wait group
func (c *ConMan[T]) reserveOne() {
	c.buffer <- nil
	c.wg.Add(1)
}

// releaseOne decrements wait group and releases a slot from concurrency buffer
func (c *ConMan[T]) releaseOne() {
	c.wg.Done()
	<-c.buffer
}

// executeTask runs a single task and handles its result or error
func (c *ConMan[T]) executeTask(ctx context.Context, t Task[T]) {
	op, err := t.Execute(ctx)
	if err == nil {
		c.withLock(func() {
			c.outputs = append(c.outputs, op)
		})
		return
	}

	if er, ok := err.(*RetriableError); ok {
		c.retry(ctx, t, er.RetryConfig)
		return
	}

	c.withLock(func() {
		c.errors = append(c.errors, err)
	})
}

// calculateDelay computes the delay before the next retry attempt
func (c *ConMan[T]) calculateDelay(attempt int, config *RetryConfig) time.Duration {
	delay := float64(config.InitialDelay) * math.Pow(config.BackoffFactor, float64(attempt))
	if int64(delay) > config.MaxDelay {
		delay = float64(config.MaxDelay)
	}
	if config.Jitter {
		delay = delay * rand.Float64()
	}
	return time.Duration(delay) * time.Millisecond
}

// waitForNextAttempt waits for the calculated delay before the next retry attempt
func (c *ConMan[T]) waitForNextAttempt(ctx context.Context, attempt int, config *RetryConfig) error {
	timer := time.NewTimer(c.calculateDelay(attempt, config))
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// retry attempts to execute a task up to maxRetries times
func (c *ConMan[T]) retry(ctx context.Context, t Task[T], config *RetryConfig) {
	if config == nil {
		return
	}
	var err error
	for attempts := range config.MaxAttempts {
		if err = c.waitForNextAttempt(ctx, attempts, config); err != nil {
			break
		}
		var opp T
		opp, err = t.Execute(ctx)
		if err == nil {
			c.withLock(func() {
				c.outputs = append(c.outputs, opp)
			})
			return
		}
	}
	if err != nil {
		c.withLock(func() {
			c.errors = append(c.errors, err)
		})
	}
}

// withLock executes a function while holding the mutex lock for thread safety
func (c *ConMan[T]) withLock(fn func()) {
	c.mu.Lock()
	defer c.mu.Unlock()
	fn()
}
