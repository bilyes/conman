# ConMan

ConMan is a concurrency manager for Go that allows setting a limit to the number of
tasks that can run concurrently. It provides an intuitive interface for defining
and concurrently running any type of tasks.

## Usage

Define a `Task`, which is a `stuct` implementing the `Execute` function. Example:

```go
type sum struct {
    op1 int
    op2 int
}

func (s *sum) Execute(ctx context.Context) (int, error) {
    return s.op1 + s.op2, nil
}
```

Then, create a new Concurrency Manager meant to run tasks that return an `int` value, with a
concurrency limit. Example:

```go
cm := conman.New[int](5) // concurrency limit of 5
```

Finally, run as many tasks as needed. Example:

```go
var err error
err = cm.Run(ctx, &sum{op1: 234, op2: 987})
// handle error ...
err = cm.Run(ctx, &sum{op1: 3455, op2: 200})
// handle error ...
// ...
err = cm.Run(ctx, &sum{op1: 905, op2: 7329})
// handle error ...
```

You can wait for all the tasks to complete before moving on using `cm.Wait()`.

The outputs from all the tasks are collected in `cm.Outputs()`, and errors can
be retrieved via `cm.Errors()`.

If the context `ctx` is cancelled for whatever reason, all subsequent calls to `cm.Run()` will
return an error about context cancellation.

## Retries

To automatically retry a task when it fails, the `Execute` function must return a pointer to a
`RetriableError` object. This object contains the original error and supports configurable retry
strategies.

### Retry Strategies

ConMan provides three built-in retry strategies:

#### Exponential Backoff (Default)
Delays increase exponentially with each retry attempt, with jitter.

```go
type flakyTask struct {
    runCount int
}

func (t *flakyTask) Execute(ctx context.Context) (int, error) {
    if t.runCount < 2 {
        t.runCount++
        // Retry with exponential backoff
        return -1, &RetriableError{Err: fmt.Errorf("Try again")}.WithExponentialBackoff()
    }
    return 42, nil
}
```

#### Linear Backoff
Delays increase linearly with each retry attempt.

```go
err := &RetriableError{Err: fmt.Errorf("Try again")}
return -1, err.WithLinearBackoff()
```

#### No Backoff
Immediate retries without delays.

```go
err := &RetriableError{Err: fmt.Errorf("Try again")}
return -1, err.WithNoBackoff()
```

### Custom Retry Configuration

Each retry strategy can be customized through the `RetryConfig`:

```go
type sum struct {
    op1 int
    op2 int
    runCount int
}

func (s *sum) Execute(ctx context.Context) (int, error) {
    if s.runCount < 2 {
        s.runCount++
        // Custom retry configuration
        err := &RetriableError{Err: fmt.Errorf("Try again")}
        err.RetryConfig = &RetryConfig{
            MaxAttempts:   3,
            InitialDelay:  100,  // milliseconds
            BackoffFactor: 2.0,
            MaxDelay:      5000, // milliseconds
            Jitter:        true,
        }
        return -1, err
    }
    return s.op1 + s.op2, nil
}

ctx, cancel := context.WithCancel(context.Background())
defer cancel()

cm := conman.New[int](5)
cm.Run(ctx, &sum{op1: 234, op2: 987})
cm.Run(ctx, &sum{op1: 3455, op2: 200})
```

## Complete Example

Here's a complete example of running multiple Fibonacci calculations
concurrently using ConMan with a concurrency limit of 2.

```go
package main

import (
    "context"
    "fmt"
    "log"
    "time"

    "github.com/bilyes/conman"
)

type slowFibo struct {
    operand int
}

func (s *slowFibo) fibonacci(i int) int {
    if i == 0 || i == 1 {
        return i
    }
    return s.fibonacci(i-1) + s.fibonacci(i-2)
}

func (s *slowFibo) Execute(ctx context.Context) (int, error) {
    // Long process...
    time.Sleep(2 * time.Second)
    switch {
    case <-ctx.Done():
        return -1, ctx.Err()
    default:
        return s.fibonacci(s.operand), nil
    }
}

func main() {
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()

    // Create a concurrency manager with a limit of 2.
    // This means that the total number of concurrently running
    // tasks will never exceed 2.
    cm := conman.New[int](2)

    for _, op := range []int{5, 8, 13, 16} {
        // Dispatch task executions with the context ctx
        if err := cm.Run(ctx, &slowFibo{operand: op}); err != nil {
            // There was an error with dispatching the task execution.
            // This is not an error caused by the execution itself. Those errors are handled
            // by ConMan internally and are accessible through the Errors() function.
            fmt.Printf("Error for operand %s: %v", err)
        }
    }

    // Wait until all tasks are completed
    cm.Wait()

    // Check if there were any errors
    if errs := cm.Errors(); len(errs) > 0 {
        log.Fatalf("There were calculation errors: %v", errs)
    }

    // Print the results
    fmt.Printf("Here are the results: %v", cm.Outputs())
}
```
