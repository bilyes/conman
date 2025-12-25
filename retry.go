package conman

// RetryConfig defines the retry behavior for operations that may fail temporarily.
// It includes parameters for controlling the number of attempts, delays, and backoff strategy.
type RetryConfig struct {
	MaxAttempts   int     // Maximum number of retry attempts
	InitialDelay  int64   // Initial delay in milliseconds
	BackoffFactor float64 // Multiplier for exponential backoff
	MaxDelay      int64   // Maximum delay in milliseconds
	Jitter        bool    // Whether to add random jitter to delays
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

// WithExponentialBackoff configures the error to use exponential backoff retry strategy.
// Returns the RetriableError for method chaining.
func (e *RetriableError) WithExponentialBackoff() *RetriableError {
	e.RetryConfig = ExponentialBackoffRetryPolicy{}.RetryConfig()
	return e
}

// WithLinearBackoff configures the error to use linear backoff retry strategy.
// Returns the RetriableError for method chaining.
func (e *RetriableError) WithLinearBackoff() *RetriableError {
	e.RetryConfig = LinearBackoffRetryPolicy{}.RetryConfig()
	return e
}

// WithNoBackoff configures the error to use immediate retries without delays.
// Returns the RetriableError for method chaining.
func (e *RetriableError) WithNoBackoff() *RetriableError {
	e.RetryConfig = NoBackoffRetryPolicy{}.RetryConfig()
	return e
}

// RetryPolicy defines an interface for different retry strategies.
type RetryPolicy interface {
	RetryConfig() *RetryConfig
}

// ExponentialBackoffRetryPolicy implements exponential backoff with jitter.
// Delays increase exponentially with each retry attempt.
type ExponentialBackoffRetryPolicy struct{}

// RetryConfig returns the retry configuration for exponential backoff.
func (e ExponentialBackoffRetryPolicy) RetryConfig() *RetryConfig {
	return &RetryConfig{
		MaxAttempts:   5,
		InitialDelay:  100, // 100 milliseconds
		BackoffFactor: 2.0,
		MaxDelay:      5000, // 5 seconds
		Jitter:        true,
	}
}

// LinearBackoffRetryPolicy implements linear backoff.
// Delays increase linearly with each retry attempt.
type LinearBackoffRetryPolicy struct{}

// RetryConfig returns the retry configuration for linear backoff.
func (l LinearBackoffRetryPolicy) RetryConfig() *RetryConfig {
	return &RetryConfig{
		MaxAttempts:   5,
		InitialDelay:  200, // 200 milliseconds
		BackoffFactor: 1.0,
		MaxDelay:      2000, // 2 seconds
		Jitter:        false,
	}
}

// NoBackoffRetryPolicy implements immediate retries without delays.
type NoBackoffRetryPolicy struct{}

// RetryConfig returns the retry configuration for immediate retries.
func (n NoBackoffRetryPolicy) RetryConfig() *RetryConfig {
	return &RetryConfig{
		MaxAttempts:   3,
		InitialDelay:  0,
		BackoffFactor: 0.0,
		MaxDelay:      0,
		Jitter:        false,
	}
}
