// Author: Ilyess Bachiri
// Copyright (c) 2025-present Ilyess Bachiri

package conman

import "fmt"

// RetryConfig defines the retry behavior for operations that may fail temporarily.
// It includes parameters for controlling the number of attempts, delays, and backoff strategy.
type RetryConfig struct {
	MaxAttempts   int     // Maximum number of retry attempts
	InitialDelay  int64   // Initial delay in milliseconds
	BackoffFactor float64 // Multiplier for exponential backoff
	MaxDelay      int64   // Maximum delay in milliseconds
	Jitter        bool    // Whether to add random jitter to delays
}

// validate checks the validity of the RetryConfig fields.
// It ensures that all parameters have valid values and logical relationships.
// Returns an error if any validation fails, otherwise returns nil.
func (rc *RetryConfig) validate() error {
	if rc.MaxAttempts <= 0 {
		return fmt.Errorf("MaxAttempts must be positive, got %d", rc.MaxAttempts)
	}
	if rc.InitialDelay < 0 {
		return fmt.Errorf("InitialDelay cannot be negative, got %d", rc.InitialDelay)
	}
	if rc.MaxDelay < 0 {
		return fmt.Errorf("MaxDelay cannot be negative, got %d", rc.MaxDelay)
	}
	if rc.MaxDelay > 0 && rc.InitialDelay > rc.MaxDelay {
		return fmt.Errorf("InitialDelay (%d) cannot be greater than MaxDelay (%d)",
			rc.InitialDelay, rc.MaxDelay)
	}
	if rc.BackoffFactor < 0 {
		return fmt.Errorf("BackoffFactor cannot be negative, got %f", rc.BackoffFactor)
	}
	if rc.BackoffFactor == 0.0 && rc.InitialDelay > 0 && rc.MaxAttempts > 1 {
		// This creates immediate zero delays after first attempt
		return fmt.Errorf("BackoffFactor of 0.0 with non-zero InitialDelay will result in zero delays")
	}
	return nil
}

// RetriableError is an error type that indicates a task should be retried.
// It includes an embedded RetryConfig to specify the retry strategy.
type RetriableError struct {
	Err         error
	RetryConfig *RetryConfig
}

// Error returns the error message of the underlying error.
func (e *RetriableError) Error() string {
	return e.Err.Error()
}

func (e *RetriableError) WithRetryConfig(config *RetryConfig) (*RetriableError, error) {
	if err := config.validate(); err != nil {
		return nil, err
	}
	e.RetryConfig = config
	return e, nil
}

// WithExponentialBackoff configures the error to use exponential backoff retry strategy.
// Returns the RetriableError for method chaining.
func (e *RetriableError) WithExponentialBackoff() *RetriableError {
	e.RetryConfig = &RetryConfig{
		MaxAttempts:   5,
		InitialDelay:  100, // 100 milliseconds
		BackoffFactor: 2.0,
		MaxDelay:      5000, // 5 seconds
		Jitter:        true,
	}
	return e
}

// WithLinearBackoff configures the error to use linear backoff retry strategy.
// Returns the RetriableError for method chaining.
func (e *RetriableError) WithLinearBackoff() *RetriableError {
	e.RetryConfig = &RetryConfig{
		MaxAttempts:   5,
		InitialDelay:  200, // 200 milliseconds
		BackoffFactor: 1.0,
		MaxDelay:      2000, // 2 seconds
		Jitter:        false,
	}
	return e
}

// WithNoBackoff configures the error to use immediate retries without delays.
// Returns the RetriableError for method chaining.
func (e *RetriableError) WithNoBackoff() *RetriableError {
	e.RetryConfig = &RetryConfig{
		MaxAttempts:   3,
		InitialDelay:  0,
		BackoffFactor: 0.0,
		MaxDelay:      0,
		Jitter:        false,
	}
	return e
}
