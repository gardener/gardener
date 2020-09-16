// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package webhook

import (
	"context"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/runtime/inject"
)

// Validator validates objects.
type Validator interface {
	Validate(ctx context.Context, new, old runtime.Object) error
}

type validationWrapper struct {
	Validator
}

// Mutate implements the `Mutator` interface and calls the `Validate` function of the underlying validator.
func (d *validationWrapper) Mutate(ctx context.Context, new, old runtime.Object) error {
	return d.Validate(ctx, new, old)
}

// InjectFunc calls the inject.Func on the handler mutators.
func (d *validationWrapper) InjectFunc(f inject.Func) error {
	if err := f(d.Validator); err != nil {
		return errors.Wrap(err, "could not inject into the validator")
	}
	return nil
}

func hybridValidator(val Validator) Mutator {
	return &validationWrapper{val}
}
