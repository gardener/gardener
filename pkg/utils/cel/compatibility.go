// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

// Package cel provides a small helper to compile and evaluate the CEL compatibility rules of an
// ExtensionProfile version (GEP-0078) against a Shoot. The CEL environment binds a single variable
// `shoot` to the Shoot being admitted.
//
// The `shoot` variable is bound as a dynamic type and evaluated against the unstructured form of the
// Shoot. This is a deliberate trade-off for the alpha of GEP-0078: rules may reference arbitrary Shoot
// fields without the helper carrying the full Shoot schema as a CEL DeclType, at the cost of no
// compile-time field checking — a rule referencing a non-existent field compiles but errors at
// evaluation time. Callers must treat an evaluation error as a rejection with a clear message.
package cel

import (
	"context"
	"fmt"
	"sync"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/version"
	"k8s.io/apiserver/pkg/cel/environment"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
)

// shootVariableName is the name of the CEL variable bound to the Shoot.
const shootVariableName = "shoot"

// Compiler compiles and caches ExtensionProfile compatibility CEL rules.
type Compiler struct {
	env   *cel.Env
	mutex sync.RWMutex
	cache map[string]*CompiledRule
}

// CompiledRule is a compiled compatibility CEL rule, ready to be evaluated against a Shoot.
type CompiledRule struct {
	rule    string
	program cel.Program
}

// NewCompiler returns a Compiler whose CEL environment binds the variable `shoot`.
func NewCompiler() (*Compiler, error) {
	baseEnv := environment.MustBaseEnvSet(environment.DefaultCompatibilityVersion())

	extended, err := baseEnv.Extend(environment.VersionedOptions{
		// Recorded as 1.0 since the variable is available in the first version that supports this feature.
		IntroducedVersion: version.MajorMinor(1, 0),
		EnvOptions: []cel.EnvOption{
			cel.Variable(shootVariableName, cel.DynType),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to extend CEL environment: %w", err)
	}

	return &Compiler{
		env:   extended.NewExpressionsEnv(),
		cache: make(map[string]*CompiledRule),
	}, nil
}

// Compile compiles a single CEL rule, enforcing that it returns a boolean. Compiled programs are
// cached by rule string, so repeated compilation of the same rule is cheap.
func (c *Compiler) Compile(rule string) (*CompiledRule, error) {
	c.mutex.RLock()
	compiled, ok := c.cache[rule]
	c.mutex.RUnlock()
	if ok {
		return compiled, nil
	}

	ast, issues := c.env.Compile(rule)
	if issues != nil && issues.Err() != nil {
		return nil, fmt.Errorf("failed to compile CEL rule %q: %w", rule, issues.Err())
	}
	if ast.OutputType() != cel.BoolType {
		return nil, fmt.Errorf("CEL rule %q must evaluate to a boolean, but evaluates to %v", rule, ast.OutputType())
	}

	program, err := c.env.Program(ast)
	if err != nil {
		return nil, fmt.Errorf("failed to build CEL program for rule %q: %w", rule, err)
	}

	compiled = &CompiledRule{rule: rule, program: program}

	c.mutex.Lock()
	c.cache[rule] = compiled
	c.mutex.Unlock()

	return compiled, nil
}

// EvalBool evaluates the compiled rule against the given Shoot and returns its boolean result.
func (r *CompiledRule) EvalBool(ctx context.Context, shoot *gardencorev1beta1.Shoot) (bool, error) {
	unstructuredShoot, err := toUnstructured(shoot)
	if err != nil {
		return false, err
	}

	val, _, err := r.program.ContextEval(ctx, map[string]any{shootVariableName: unstructuredShoot})
	if err != nil {
		return false, fmt.Errorf("failed to evaluate CEL rule %q: %w", r.rule, err)
	}

	result, err := toBool(val)
	if err != nil {
		return false, fmt.Errorf("CEL rule %q did not return a boolean: %w", r.rule, err)
	}

	return result, nil
}

// toUnstructured converts the Shoot to its unstructured (map) representation for CEL evaluation.
func toUnstructured(shoot *gardencorev1beta1.Shoot) (map[string]any, error) {
	out, err := runtime.DefaultUnstructuredConverter.ToUnstructured(shoot)
	if err != nil {
		return nil, fmt.Errorf("failed to convert shoot to unstructured for CEL evaluation: %w", err)
	}
	return out, nil
}

// toBool extracts a Go bool from a CEL evaluation result, rejecting non-boolean values.
func toBool(val ref.Val) (bool, error) {
	if val == nil {
		return false, fmt.Errorf("nil evaluation result")
	}
	if val.Type() != types.BoolType {
		return false, fmt.Errorf("expected a boolean result, got %v", val.Type())
	}
	result, ok := val.Value().(bool)
	if !ok {
		return false, fmt.Errorf("could not convert evaluation result to boolean")
	}
	return result, nil
}
