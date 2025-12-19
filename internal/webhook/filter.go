// Copyright Contributors to the KubeTask project

package webhook

import (
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
)

// CELFilter evaluates CEL expressions against webhook payloads.
type CELFilter struct {
	// cache stores compiled CEL programs keyed by expression
	cache sync.Map
}

// NewCELFilter creates a new CEL filter.
func NewCELFilter() *CELFilter {
	return &CELFilter{}
}

// compiledProgram holds a compiled CEL program and its environment.
type compiledProgram struct {
	program cel.Program
	env     *cel.Env
}

// Evaluate evaluates a CEL expression against the webhook payload and headers.
// Returns true if the expression evaluates to true, false otherwise.
// If the expression is empty, returns true (no filter means accept all).
func (f *CELFilter) Evaluate(expression string, payload map[string]interface{}, headers http.Header) (bool, error) {
	if expression == "" {
		return true, nil
	}

	// Get or compile the program
	prog, err := f.getOrCompile(expression)
	if err != nil {
		return false, fmt.Errorf("failed to compile CEL expression: %w", err)
	}

	// Prepare headers as lowercase map
	headerMap := make(map[string]string)
	for k, v := range headers {
		if len(v) > 0 {
			headerMap[strings.ToLower(k)] = v[0]
		}
	}

	// Evaluate the expression
	result, _, err := prog.program.Eval(map[string]interface{}{
		"body":    payload,
		"headers": headerMap,
	})
	if err != nil {
		return false, fmt.Errorf("failed to evaluate CEL expression: %w", err)
	}

	// Convert result to bool
	if result.Type() != types.BoolType {
		return false, fmt.Errorf("CEL expression must return bool, got %s", result.Type())
	}

	boolVal, ok := result.Value().(bool)
	if !ok {
		return false, fmt.Errorf("CEL expression result is not a bool")
	}
	return boolVal, nil
}

// getOrCompile returns a cached compiled program or compiles a new one.
func (f *CELFilter) getOrCompile(expression string) (*compiledProgram, error) {
	// Check cache
	if cached, ok := f.cache.Load(expression); ok {
		prog, ok := cached.(*compiledProgram)
		if !ok {
			return nil, fmt.Errorf("invalid cached program type")
		}
		return prog, nil
	}

	// Create CEL environment with webhook-specific variables
	env, err := cel.NewEnv(
		cel.Variable("body", cel.DynType),
		cel.Variable("headers", cel.MapType(cel.StringType, cel.StringType)),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create CEL environment: %w", err)
	}

	// Parse and check the expression
	ast, issues := env.Compile(expression)
	if issues != nil && issues.Err() != nil {
		return nil, fmt.Errorf("CEL compilation error: %w", issues.Err())
	}

	// Verify the expression returns a bool
	if ast.OutputType() != cel.BoolType {
		// Allow dynamic types that might resolve to bool at runtime
		if ast.OutputType() != cel.DynType {
			return nil, fmt.Errorf("CEL expression must return bool, got %s", ast.OutputType())
		}
	}

	// Create the program
	program, err := env.Program(ast,
		cel.EvalOptions(cel.OptOptimize),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create CEL program: %w", err)
	}

	compiled := &compiledProgram{
		program: program,
		env:     env,
	}

	// Cache the compiled program
	f.cache.Store(expression, compiled)

	return compiled, nil
}

// ClearCache clears the compiled program cache.
// Useful for testing or when expressions change.
func (f *CELFilter) ClearCache() {
	f.cache = sync.Map{}
}

// ValidateExpression validates a CEL expression without evaluating it.
// Returns nil if the expression is valid, an error otherwise.
func ValidateExpression(expression string) error {
	if expression == "" {
		return nil
	}

	env, err := cel.NewEnv(
		cel.Variable("body", cel.DynType),
		cel.Variable("headers", cel.MapType(cel.StringType, cel.StringType)),
	)
	if err != nil {
		return fmt.Errorf("failed to create CEL environment: %w", err)
	}

	ast, issues := env.Compile(expression)
	if issues != nil && issues.Err() != nil {
		return fmt.Errorf("CEL compilation error: %w", issues.Err())
	}

	// Verify the expression returns a bool or dynamic type
	if ast.OutputType() != cel.BoolType && ast.OutputType() != cel.DynType {
		return fmt.Errorf("CEL expression must return bool, got %s", ast.OutputType())
	}

	return nil
}
