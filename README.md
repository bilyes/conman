# ConMan, A Concurrency Manager for Go

ConMan is a concurrency manager that allows you to set a limit to the number of
go-routines you want to run concurrently. It provides an intuitive interface for
defining and running any type of tasks concurrently.

### Example

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

func (s *slowFibo) Execute() (interface{}, error) {
    // Long process...
    time.Sleep(2 * time.Second)
    return s.fibonacci(s.operand), nil
}

func main() {
    // Create a concurrency manager with a limit of 2.
    // This means that the total number of concurrently running
    // tasks will never exceed 2.
    cm := conman.New(2)

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
