// Author: Ilyess Bachiri
// Copyright (c) 2021-present Ilyess Bachiri

// Package conman, a concurrency management package
package conman

import (
	"context"
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

// New creates a new ConMan instance
func New[T any](concurrencyLimit int64) ConMan[T] {
	return ConMan[T]{buffer: make(chan any, concurrencyLimit)}
}

// Task an interface that defines task execution
type Task[T any] interface {
	Execute(ctx context.Context) (T, error)
}

// Run runs a task function
// If the concurrency limit is reached, it waits until a running
// task is done.
//
// A task function must not take in any parameters, and must return
// a value of type T and an error.
// e.g.: func () (T, error) {}
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

// Wait suspends execution until all running tasks are done
func (c *ConMan[T]) Wait() {
	c.wg.Wait()
}

// Outputs returns a slice of returned values from all the tasks
// that did not return an error.
func (c *ConMan[T]) Outputs() []T {
	var result []T
	c.withLock(func() {
		result = c.outputs
	})
	return result
}

// Errors returns a slice of errors that were returned
// by all the tasks run by the Run function.
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
