// Author: Ilyess Bachiri
// Copyright (c) 2021-present Ilyess Bachiri

// Package conman, a concurrency management package
package conman

import "sync"

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
	Execute() (T, error)
}

// Run runs a task function
// If the concurrency limit is reached, it waits until a running
// task is done.
//
// A task function must not take in any parameters, and must return
// a value of type T and an error.
// e.g.: func () (T, error) {}
func (c *ConMan[T]) Run(t Task[T]) {
	c.reserveOne()
	go func() {
		defer c.releaseOne()

		op, err := t.Execute()
		if err != nil {
			c.errors = append(c.errors, err)
		} else {
			c.outputs = append(c.outputs, op)
		}
	}()
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
