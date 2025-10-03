package example

import (
	"context"
	"testing"

	examplev1 "github.com/andrewstucki/protoc-states/example/gen/example/v1"
	"github.com/andrewstucki/protoc-states/workflows"
	"github.com/stretchr/testify/require"
)

func TestExample(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	processor := workflows.NewWorkflowProcessorBuilder().Register(examplev1.NewStateMachineWorkflowRegistration(
		&ExampleWorkflowHandler{},
	)).Build()

	require.NoError(t, processor.Start(ctx))

	futureOne, err := processor.RunWorkflow(ctx, examplev1.StateMachineWorkflow, examplev1.StateMachine{
		A: "A",
		B: "B",
		C: "C",
	})
	require.NoError(t, err)

	var outputOne examplev1.StateMachine
	require.NoError(t, futureOne.WaitFor(ctx, &outputOne))

	require.Equal(t, "A Foo", outputOne.A)
	require.Equal(t, "B Bar", outputOne.B)
	require.Equal(t, "C Baz", outputOne.C)

	futureTwo, err := processor.RunWorkflow(ctx, examplev1.StateMachineWorkflow)
	require.NoError(t, err)

	var outputTwo examplev1.StateMachine
	require.NoError(t, futureTwo.WaitFor(ctx, &outputTwo))

	require.Equal(t, "Foo", outputTwo.A)
	require.Equal(t, "Bar", outputTwo.B)
	require.Equal(t, "Baz", outputTwo.C)

	require.NoError(t, processor.Shutdown(ctx))
}
