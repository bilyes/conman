// Author: Ilyess Bachiri
// Copyright (c) 2021-present Ilyess Bachiri

// Package conman, a concurrency management package
package conman

import "sync"

// ConMan a structure to manage multiple tasks running
// concurrently while ensuring the total number of running
// tasks doesn't exceed a certain concurrency limit
type ConMan struct {
	wg      sync.WaitGroup
	errors  []error
	outputs []any
	buffer  chan any
}

// New creates a new ConMan instance
func New(concurrencyLimit int64) ConMan {
	return ConMan{buffer: make(chan any, concurrencyLimit)}
}

// Task an interface that defines task execution
type Task interface {
	Execute() (any, error)
}

// Run runs a task function
// If the concurrency limit is reached, it waits until a running
// task is done.
//
// A task function must not take in any parameters, and must return
// a any-error pair
// e.g.: func () (any, error) {}
func (c *ConMan) Run(t Task) {
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
func (c *ConMan) Wait() {
	c.wg.Wait()
}

// Outputs returns a slice of returned values from all the tasks
// that did not return an error
func (c *ConMan) Outputs() []any {
	return c.outputs
}

// Errors returns a slice of errors that were returned
// by all the tasks run by the Run function
func (c *ConMan) Errors() []error {
	return c.errors
}

func (c *ConMan) reserveOne() {
	c.buffer <- nil
	c.wg.Add(1)
}

func (c *ConMan) releaseOne() {
	c.wg.Done()
	<-c.buffer
}
