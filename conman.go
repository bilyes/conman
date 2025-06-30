// Author: Ilyess Bachiri
// Copyright (c) 2021-present Ilyess Bachiri

// Package conman, a concurrency management package
package conman

import (
	"context"
	"sync"
)

// RetriableError is an error type that indicates a task should be retried.
// It contains the original error and the maximum number of retries allowed.
type RetriableError struct {
	Err        error
	MaxRetries int
}

func (e *RetriableError) Error() string {
	return e.Err.Error()
}

// ConMan a structure to manage multiple tasks running
// concurrently while ensuring the total number of running
// tasks doesn't exceed a certain concurrency limit
type ConMan[T any] struct {
	wg      sync.WaitGroup
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
// that did not return an error
func (c *ConMan[T]) Outputs() []T {
	return c.outputs
}

// Errors returns a slice of errors that were returned
// by all the tasks run by the Run function
func (c *ConMan[T]) Errors() []error {
	return c.errors
}

func (c *ConMan[T]) reserveOne() {
	c.buffer <- nil
	c.wg.Add(1)
}

func (c *ConMan[T]) releaseOne() {
	c.wg.Done()
	<-c.buffer
}

func (c *ConMan[T]) executeTask(ctx context.Context, t Task[T]) {
	op, err := t.Execute(ctx)
	if err == nil {
		c.outputs = append(c.outputs, op)
		return
	}

	if er, ok := err.(*RetriableError); ok {
		c.retry(ctx, t, er.MaxRetries)
		return
	}

	c.errors = append(c.errors, err)
}

func (c *ConMan[T]) retry(ctx context.Context, t Task[T], maxRetries int) {
	retries := 0
	var err error
	for retries < maxRetries {
		var opp T
		opp, err = t.Execute(ctx)
		if err == nil {
			c.outputs = append(c.outputs, opp)
			return
		}
		retries++
	}
	if err != nil {
		c.errors = append(c.errors, err)
	}
}
