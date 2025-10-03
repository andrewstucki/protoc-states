package workflows

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/microsoft/durabletask-go/api"
	"github.com/microsoft/durabletask-go/backend"
	"github.com/microsoft/durabletask-go/task"
)

type noopLogger struct{}

func NoopLogger() backend.Logger {
	return noopLogger{}
}

func (noopLogger) Debug(...any)          {}
func (noopLogger) Debugf(string, ...any) {}
func (noopLogger) Error(...any)          {}
func (noopLogger) Errorf(string, ...any) {}
func (noopLogger) Info(...any)           {}
func (noopLogger) Infof(string, ...any)  {}
func (noopLogger) Warn(...any)           {}
func (noopLogger) Warnf(string, ...any)  {}

type RetryPolicy struct {
	MaxAttempts          int
	InitialRetryInterval time.Duration
	BackoffCoefficient   float64
	MaxRetryInterval     time.Duration
	RetryTimeout         time.Duration
	Handle               func(error) bool
}

type WorkflowStep[T any] struct {
	Name    string
	Retries *RetryPolicy
	Fn      func(io *T) error
	Next    *WorkflowStep[T]
}

func (s *WorkflowStep[T]) ToActivity(ctx task.ActivityContext) (any, error) {
	var input T
	if err := ctx.GetInput(&input); err != nil {
		return nil, err
	}

	if err := s.Fn(&input); err != nil {
		return nil, err
	}
	return input, nil
}

type Workflow[T any] struct {
	Name       string
	Entrypoint *WorkflowStep[T]
}

type NamedActivity struct {
	Name string
	Fn   task.Activity
}

func (w *Workflow[T]) Activities() []NamedActivity {
	var activities []NamedActivity
	activity := w.Entrypoint
	for activity != nil {
		activities = append(activities, NamedActivity{
			Name: activity.Name,
			Fn:   activity.ToActivity,
		})
		activity = activity.Next
	}
	return activities
}

func (w *Workflow[T]) ToOrchestration(ctx *task.OrchestrationContext) (any, error) {
	var input T
	var output T
	activity := w.Entrypoint
	if err := ctx.GetInput(&input); err != nil {
		return nil, err
	}
	for activity != nil {
		if activity.Retries != nil {
			if err := ctx.CallActivity(activity.Name, task.WithActivityInput(input), task.WithActivityRetryPolicy(&task.RetryPolicy{
				MaxAttempts:          activity.Retries.MaxAttempts,
				InitialRetryInterval: activity.Retries.InitialRetryInterval,
				BackoffCoefficient:   activity.Retries.BackoffCoefficient,
				MaxRetryInterval:     activity.Retries.MaxRetryInterval,
				RetryTimeout:         activity.Retries.RetryTimeout,
				Handle:               activity.Retries.Handle,
			})).Await(&output); err != nil {
				return nil, err
			}
		} else {
			if err := ctx.CallActivity(activity.Name, task.WithActivityInput(input)).Await(&output); err != nil {
				return nil, err
			}
		}
		activity = activity.Next
		input = output
	}

	return input, nil
}

type Future struct {
	id     api.InstanceID
	client backend.TaskHubClient
}

func (f *Future) ID() string {
	return string(f.id)
}

func (f *Future) Wait(ctx context.Context) (string, error) {
	metadata, err := f.client.WaitForOrchestrationCompletion(ctx, f.id)
	if err != nil {
		return "", err
	}

	if metadata.FailureDetails != nil {
		return "", fmt.Errorf("%s: %s", metadata.FailureDetails.ErrorType, metadata.FailureDetails.ErrorMessage)
	}

	return metadata.SerializedOutput, nil
}

func (f *Future) WaitFor(ctx context.Context, data any) error {
	metadata, err := f.client.WaitForOrchestrationCompletion(ctx, f.id)
	if err != nil {
		return err
	}

	if metadata.FailureDetails != nil {
		return fmt.Errorf("%s: %s", metadata.FailureDetails.ErrorType, metadata.FailureDetails.ErrorMessage)
	}

	return json.Unmarshal([]byte(metadata.SerializedOutput), data)
}

type Registration struct {
	name          string
	workflow      WorkflowFn
	orchestration task.Orchestrator
	activities    []NamedActivity
}

type WorkflowFn func(context.Context, backend.TaskHubClient, ...any) (*Future, error)

func NewRegistration[T any](w *Workflow[T]) Registration {
	return Registration{
		name: w.Name,
		workflow: func(ctx context.Context, client backend.TaskHubClient, inputs ...any) (*Future, error) {
			if len(inputs) > 1 {
				return nil, errors.New("cannot pass more than a single input")
			}

			var input T
			if len(inputs) > 0 {
				cast, ok := inputs[0].(T)
				if !ok {
					return nil, errors.New("input type does not match workflow input type")
				}
				input = cast
			}

			id, err := client.ScheduleNewOrchestration(ctx, w.Name, api.WithInput(input))
			if err != nil {
				return nil, err
			}

			return &Future{id: id, client: client}, nil
		},
		orchestration: w.ToOrchestration,
		activities:    w.Activities(),
	}
}

type BackendFactory func(backend.Logger) backend.Backend

type WorkflowProcessor struct {
	registry  *task.TaskRegistry
	logger    backend.Logger
	executor  backend.Executor
	backend   backend.Backend
	client    backend.TaskHubClient
	worker    backend.TaskHubWorker
	workflows map[string]Registration
}

type WorkflowProcessorBuilder struct {
	factory       BackendFactory
	logger        backend.Logger
	registrations []Registration
}

func NewWorkflowProcessorBuilder() *WorkflowProcessorBuilder {
	return &WorkflowProcessorBuilder{
		factory: NewMemoryBackend,
		logger:  NoopLogger(),
	}
}

func (b *WorkflowProcessorBuilder) WithBackendFactory(factory BackendFactory) *WorkflowProcessorBuilder {
	b.factory = factory
	return b
}

func (b *WorkflowProcessorBuilder) WithLogger(logger backend.Logger) *WorkflowProcessorBuilder {
	b.logger = logger
	return b
}

func (b *WorkflowProcessorBuilder) Register(registrations ...Registration) *WorkflowProcessorBuilder {
	b.registrations = append(b.registrations, registrations...)
	return b
}

func (b *WorkflowProcessorBuilder) Build() *WorkflowProcessor {
	registry := task.NewTaskRegistry()
	be := b.factory(b.logger)

	workflows := make(map[string]Registration)

	for _, registration := range b.registrations {
		registry.AddOrchestratorN(registration.name, registration.orchestration)
		for _, activity := range registration.activities {
			registry.AddActivityN(activity.Name, activity.Fn)
		}
		workflows[registration.name] = registration
	}

	return &WorkflowProcessor{
		registry:  registry,
		logger:    b.logger,
		executor:  task.NewTaskExecutor(registry),
		backend:   be,
		client:    backend.NewTaskHubClient(be),
		workflows: workflows,
	}
}

func (p *WorkflowProcessor) Start(ctx context.Context) error {
	if err := p.backend.Start(ctx); err != nil {
		return err
	}
	orchestrationWorker := backend.NewOrchestrationWorker(p.backend, p.executor, p.logger)
	activityWorker := backend.NewActivityTaskWorker(p.backend, p.executor, p.logger)
	p.worker = backend.NewTaskHubWorker(p.backend, orchestrationWorker, activityWorker, p.logger)
	return p.worker.Start(ctx)
}

func (p *WorkflowProcessor) RunWorkflow(ctx context.Context, name string, inputs ...any) (*Future, error) {
	reg, ok := p.workflows[name]
	if !ok {
		return nil, fmt.Errorf("workflow %q not registered", name)
	}
	return reg.workflow(ctx, p.client, inputs...)
}

func (p *WorkflowProcessor) Shutdown(ctx context.Context) error {
	err := p.backend.Stop(ctx)
	if p.worker != nil {
		return errors.Join(err, p.worker.Shutdown(ctx))
	}
	return err
}
