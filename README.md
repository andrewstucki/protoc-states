# Protoc States

`protoc-states` is a small protobuf plugin that generates Go workflow wiring from protobuf message options that describe a state machine.

The generated code is designed to be used with the `workflows` package in this repository which provides a thin orchestration abstraction on top of durabletask-go (in-memory  available). This makes it easy to register, start and run state-machine-style workflows backed by durabletask.

## How it works

1. Annotate a protobuf message with the `state.v1.Machine` extension (see `proto/state/v1/state.proto`). The Machine contains `States` which contain ordered `StateTransition` entries and optional retry policies.
2. Build and install the code generator as a protoc plugin named `protoc-gen-states`.
3. Generate your code.
4. Implement the generated `*WorkflowHandler` interface for your message type and register it with the `workflows` runtime.

Example:

```proto
import "proto/state/v1/state.proto";

message StateMachine {
	option (state.v1.machine).states = {
    transitions: [ { name: "foo" }, { name: "bar" } ]
	};

	string a = 1;
	string b = 2;
}
```

The generator produces `StateMachine.state.go` with a handler interface like:

```go
type StateMachineWorkflowHandler interface {
		Foo(io *StateMachine) error
		Bar(io *StateMachine) error
}
```

And a helper `NewStateMachineWorkflowRegistration(handler StateMachineWorkflowHandler) workflows.Registration` that can be used to register the workflow with the runtime.

## Using the generated code above

A small example using the `workflows` runtime:

```go
// implement the generated handler
type MyHandler struct{}

func (h *MyHandler) Foo(m *examplev1.StateMachine) error { /* ... */ }
func (h *MyHandler) Bar(m *examplev1.StateMachine) error { /* ... */ }

func main() {
	// register and run
	processor := workflows.NewWorkflowProcessorBuilder().Register(
		examplev1.NewStateMachineWorkflowRegistration(&MyHandler{}),
	).Build()

	ctx := context.Background()

	processor.Start(ctx) // handle error
	defer func() {
		proc.Shutdown(context.Background()) // handle error
	}()

	// run a workflow
	future, _ := proc.RunWorkflow(ctx, examplev1.StateMachineWorkflow, examplev1.StateMachine{
		// ...
	})
	// or future, _ := proc.RunWorkflow(ctx, examplev1.StateMachineWorkflow) if no input is needed

	var output examplev1.StateMachine
	future.WaitFor(ctx, &output) // handle error

	// use output
}
```

See `example/` for a small, concrete example and the generated code used by it.