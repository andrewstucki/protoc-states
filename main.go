package main

import (
	"bytes"
	"errors"
	"go/format"
	"log"
	"text/template"

	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/proto"

	statev1 "github.com/andrewstucki/protoc-states/gen/state/v1"
)

func main() {
	protogen.Options{}.Run(func(plugin *protogen.Plugin) error {
		return NewStateMachineGenerator(plugin).Run()
	})
}

type StateMachineGenerator struct {
	plugin *protogen.Plugin
}

// NewStateMachineGenerator creates a new generator for a protoc plugin invocation.
func NewStateMachineGenerator(plugin *protogen.Plugin) *StateMachineGenerator {
	return &StateMachineGenerator{
		plugin: plugin,
	}
}

// Run runs the generator.
func (g *StateMachineGenerator) Run() error {
	g.buildStateMachine()
	// outputFile := g.plugin.NewGeneratedFile("openapi.yaml", "")
	// outputFile.Write([]byte{})
	return nil
}

// buildDocumentV3 builds an StateMachine document for a plugin request.
func (g *StateMachineGenerator) buildStateMachine() {
	for _, file := range g.plugin.Files {
		g.addPaths(file)
	}
}

func (g *StateMachineGenerator) addPaths(file *protogen.File) {
	for _, message := range file.Messages {
		extension := proto.GetExtension(message.Desc.Options(), statev1.E_Machine)
		if extension != nil {
			machine := extension.(*statev1.Machine)
			if machine == nil || machine.States == nil {
				continue
			}

			if err := validateTransitions(machine.States.Transitions); err != nil {
				log.Fatalf("invalid state machine in %s: %v", message.Desc.FullName(), err)
			}

			var buffer bytes.Buffer
			if err := stateTemplate.Execute(&buffer, struct {
				Package string
				Machine string
				State   *statev1.States
			}{
				Package: string(file.GoPackageName),
				Machine: message.GoIdent.GoName,
				State:   machine.States,
			}); err != nil {
				log.Fatalf("failed to execute template for %s: %v", message.Desc.FullName(), err)
			}

			data, err := format.Source(buffer.Bytes())
			if err != nil {
				log.Fatalf("failed to format generated code for %s: %v", message.Desc.FullName(), err)
			}

			outputFile := g.plugin.NewGeneratedFile(file.GeneratedFilenamePrefix+".state.go", file.GoImportPath)
			if _, err := outputFile.Write(data); err != nil {
				log.Fatalf("failed to write generated file: %v", err)
			}
		}
	}
}

func validateTransitions(transitions []*statev1.StateTransition) error {
	seen := map[string]struct{}{}
	for _, transition := range transitions {
		if transition.Name == "" {
			return errors.New("transition 'name' field is required")
		}
		if _, ok := seen[transition.Name]; ok {
			return errors.New("duplicate transition 'name' field: " + transition.Name)
		}
		seen[transition.Name] = struct{}{}
	}

	return nil
}

func entrypoint(state *statev1.States) *statev1.StateTransition {
	if len(state.Transitions) == 0 {
		return nil
	}

	return state.Transitions[0]
}

var (
	stateTemplateString = `
package {{ .Package }}

import (
  "time"

	"github.com/andrewstucki/protoc-states/workflows"
)

{{- $machine := .Machine }}
{{- $state := .State }}
const  {{ $machine }}Workflow = "{{ $machine }}"

type {{ $machine }}WorkflowHandler interface {
{{- range $transition := $state.Transitions }}
	{{ camelCase $transition.Name }}(io *{{ $machine }}) error
{{- end }}
}

{{- range $i, $transition := $state.Transitions }}
func workflowStep{{ camelCase $transition.Name }}(handler {{ $machine }}WorkflowHandler) *workflows.WorkflowStep[{{ $machine }}]{
	return &workflows.WorkflowStep[{{ $machine }}]{
			Name: "{{ $transition.Name }}",
			Fn:   handler.{{ camelCase $transition.Name }},
			{{- if $transition.RetryPolicy }}
			Retries: &workflows.RetryPolicy{
				MaxAttempts:          {{ $transition.RetryPolicy.MaxAttempts }},
				InitialRetryInterval: {{ $transition.RetryPolicy.InitialRetryIntervalSeconds }}*time.Second,
				BackoffCoefficient:   {{ $transition.RetryPolicy.BackoffCoefficient }},
				MaxRetryInterval:     {{ $transition.RetryPolicy.MaxRetryIntervalSeconds }}*time.Second,
				RetryTimeout:         {{ $transition.RetryPolicy.RetryTimeoutSeconds }}*time.Second,
			},
			{{- else if $state.DefaultRetryPolicy }}
			Retries: &workflows.RetryPolicy{
				MaxAttempts:          {{ $state.DefaultRetryPolicy.MaxAttempts }},
				InitialRetryInterval: {{ $state.DefaultRetryPolicy.InitialRetryIntervalSeconds }}*time.Second,
				BackoffCoefficient:   {{ $state.DefaultRetryPolicy.BackoffCoefficient }},
				MaxRetryInterval:     {{ $state.DefaultRetryPolicy.MaxRetryIntervalSeconds }}*time.Second,
				RetryTimeout:         {{ $state.DefaultRetryPolicy.RetryTimeoutSeconds }}*time.Second,
			},
			{{- end }}
			{{- with $next := (next $i $state) }}
			Next: workflowStep{{ camelCase $next.Name }}(handler),
			{{- end }}
	}
}
{{- end }}

func New{{ $machine }}WorkflowRegistration(handler {{ $machine }}WorkflowHandler) workflows.Registration {
	return workflows.NewRegistration(&workflows.Workflow[{{ $machine }}]{
		Name: {{ $machine }}Workflow,
		Entrypoint: workflowStep{{ camelCase (entrypoint $state).Name }}(handler),
	})
}
`
	stateTemplate = template.Must(template.New("state").Funcs(template.FuncMap{
		"camelCase":  GoCamelCase,
		"entrypoint": entrypoint,
		"next": func(index int, state *statev1.States) *statev1.StateTransition {
			if index >= len(state.Transitions)-1 {
				return nil
			}

			return state.Transitions[index+1]
		},
	}).Parse(stateTemplateString))
)
