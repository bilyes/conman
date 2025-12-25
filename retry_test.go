package conman

import (
	"testing"
)

func TestRetryConfigValidate(t *testing.T) {
	tests := []struct {
		name        string
		config      RetryConfig
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid config",
			config: RetryConfig{
				MaxAttempts:   5,
				InitialDelay:  100,
				BackoffFactor: 2.0,
				MaxDelay:      5000,
				Jitter:        true,
			},
			expectError: false,
		},
		{
			name: "zero max attempts",
			config: RetryConfig{
				MaxAttempts:   0,
				InitialDelay:  100,
				BackoffFactor: 2.0,
				MaxDelay:      5000,
			},
			expectError: true,
			errorMsg:    "MaxAttempts must be positive, got 0",
		},
		{
			name: "negative max attempts",
			config: RetryConfig{
				MaxAttempts:   -1,
				InitialDelay:  100,
				BackoffFactor: 2.0,
				MaxDelay:      5000,
			},
			expectError: true,
			errorMsg:    "MaxAttempts must be positive, got -1",
		},
		{
			name: "negative initial delay",
			config: RetryConfig{
				MaxAttempts:   5,
				InitialDelay:  -100,
				BackoffFactor: 2.0,
				MaxDelay:      5000,
			},
			expectError: true,
			errorMsg:    "InitialDelay cannot be negative, got -100",
		},
		{
			name: "negative max delay",
			config: RetryConfig{
				MaxAttempts:   5,
				InitialDelay:  100,
				BackoffFactor: 2.0,
				MaxDelay:      -5000,
			},
			expectError: true,
			errorMsg:    "MaxDelay cannot be negative, got -5000",
		},
		{
			name: "initial delay greater than max delay",
			config: RetryConfig{
				MaxAttempts:   5,
				InitialDelay:  6000,
				BackoffFactor: 2.0,
				MaxDelay:      5000,
			},
			expectError: true,
			errorMsg:    "InitialDelay (6000) cannot be greater than MaxDelay (5000)",
		},
		{
			name: "negative backoff factor",
			config: RetryConfig{
				MaxAttempts:   5,
				InitialDelay:  100,
				BackoffFactor: -1.0,
				MaxDelay:      5000,
			},
			expectError: true,
			errorMsg:    "BackoffFactor cannot be negative, got -1.000000",
		},
		{
			name: "zero backoff factor with non-zero initial delay and multiple attempts",
			config: RetryConfig{
				MaxAttempts:   5,
				InitialDelay:  100,
				BackoffFactor: 0.0,
				MaxDelay:      5000,
			},
			expectError: true,
			errorMsg:    "BackoffFactor of 0.0 with non-zero InitialDelay will result in zero delays",
		},
		{
			name: "zero backoff factor with zero initial delay",
			config: RetryConfig{
				MaxAttempts:   5,
				InitialDelay:  0,
				BackoffFactor: 0.0,
				MaxDelay:      0,
			},
			expectError: false,
		},
		{
			name: "zero backoff factor with single attempt",
			config: RetryConfig{
				MaxAttempts:   1,
				InitialDelay:  100,
				BackoffFactor: 0.0,
				MaxDelay:      5000,
			},
			expectError: false,
		},
		{
			name: "jitter with zero initial delay",
			config: RetryConfig{
				MaxAttempts:   5,
				InitialDelay:  0,
				BackoffFactor: 2.0,
				MaxDelay:      5000,
				Jitter:        true,
			},
			expectError: false,
		},
		{
			name: "minimal valid config",
			config: RetryConfig{
				MaxAttempts:   1,
				InitialDelay:  0,
				BackoffFactor: 0.0,
				MaxDelay:      0,
				Jitter:        false,
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.validate()
			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				} else if tt.errorMsg != "" && err.Error() != tt.errorMsg {
					t.Errorf("Expected error message %q, got %q", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
			}
		})
	}
}
