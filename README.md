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

func (s *sum) Execute() (int, error) {
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
cm.Run(&sum{op1: 234, op2: 987})
cm.Run(&sum{op1: 3455, op2: 200})
// ...
cm.Run(&sum{op1: 905, op2: 7329})
```

You can wait for all the tasks to complete before moving on using `cm.Wait()`.

The outputs from all the tasks are collected in `cm.Outputs()`, and errors can
be retrieved via `cm.Errors()`.

## Complete Example

Here's a complete example of running multiple Fibonacci calculations
concurrently using ConMan with a concurrency limit of 2.

```go
package main

import (
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

func (s *slowFibo) Execute() (int, error) {
    // Long process...
    time.Sleep(2 * time.Second)
    return s.fibonacci(s.operand), nil
}

func main() {
    // Create a concurrency manager with a limit of 2.
    // This means that the total number of concurrently running
    // tasks will never exceed 2.
    cm := conman.New[int](2)

    cm.Run(&slowFibo{operand: 5})
    cm.Run(&slowFibo{operand: 8})
    cm.Run(&slowFibo{operand: 13})
    cm.Run(&slowFibo{operand: 16})

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
