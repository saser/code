package fake

import "fmt"

type invalidNameError struct {
	Name   string
	Reason string
}

func (e *invalidNameError) Error() string {
	return fmt.Sprintf(`fake: internal: name %q doesn't have format "tasks/{task}": %v`, e.Name, e.Reason)
}

type notFoundError struct {
	Name string
}

func (e *notFoundError) Error() string {
	return fmt.Sprintf(`fake: internal: task not found: %q`, e.Name)
}
