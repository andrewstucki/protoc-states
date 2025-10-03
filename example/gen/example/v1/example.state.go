package v1

import (
	"time"

	"github.com/andrewstucki/protoc-states/workflows"
)

const StateMachineWorkflow = "StateMachine"

type StateMachineWorkflowHandler interface {
	Foo(io *StateMachine) error
	Bar(io *StateMachine) error
	Baz(io *StateMachine) error
}

func workflowStepFoo(handler StateMachineWorkflowHandler) *workflows.WorkflowStep[StateMachine] {
	return &workflows.WorkflowStep[StateMachine]{
		Name: "foo",
		Fn:   handler.Foo,
		Retries: &workflows.RetryPolicy{
			MaxAttempts:          5,
			InitialRetryInterval: 1 * time.Second,
			BackoffCoefficient:   2,
			MaxRetryInterval:     10 * time.Second,
			RetryTimeout:         60 * time.Second,
		},
		Next: workflowStepBar(handler),
	}
}
func workflowStepBar(handler StateMachineWorkflowHandler) *workflows.WorkflowStep[StateMachine] {
	return &workflows.WorkflowStep[StateMachine]{
		Name: "bar",
		Fn:   handler.Bar,
		Retries: &workflows.RetryPolicy{
			MaxAttempts:          5,
			InitialRetryInterval: 1 * time.Second,
			BackoffCoefficient:   2,
			MaxRetryInterval:     10 * time.Second,
			RetryTimeout:         60 * time.Second,
		},
		Next: workflowStepBaz(handler),
	}
}
func workflowStepBaz(handler StateMachineWorkflowHandler) *workflows.WorkflowStep[StateMachine] {
	return &workflows.WorkflowStep[StateMachine]{
		Name: "baz",
		Fn:   handler.Baz,
		Retries: &workflows.RetryPolicy{
			MaxAttempts:          5,
			InitialRetryInterval: 1 * time.Second,
			BackoffCoefficient:   2,
			MaxRetryInterval:     10 * time.Second,
			RetryTimeout:         60 * time.Second,
		},
	}
}

func NewStateMachineWorkflowRegistration(handler StateMachineWorkflowHandler) workflows.Registration {
	return workflows.NewRegistration(&workflows.Workflow[StateMachine]{
		Name:       StateMachineWorkflow,
		Entrypoint: workflowStepFoo(handler),
	})
}
