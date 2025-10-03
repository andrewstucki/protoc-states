package example

import (
	"fmt"

	examplev1 "github.com/andrewstucki/protoc-states/example/gen/example/v1"
)

type ExampleWorkflowHandler struct{}

var _ examplev1.StateMachineWorkflowHandler = (*ExampleWorkflowHandler)(nil)

func (h *ExampleWorkflowHandler) Foo(machine *examplev1.StateMachine) error {
	machine.A = padAndAdd(machine.A, "Foo")
	return nil
}
func (h *ExampleWorkflowHandler) Bar(machine *examplev1.StateMachine) error {
	machine.B = padAndAdd(machine.B, "Bar")
	return nil
}
func (h *ExampleWorkflowHandler) Baz(machine *examplev1.StateMachine) error {
	machine.C = padAndAdd(machine.C, "Baz")
	return nil
}

func padAndAdd(v, add string) string {
	if v != "" {
		return fmt.Sprintf("%s %s", v, add)
	}
	return add
}
