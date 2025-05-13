// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validator

import (
	"context"
	"errors"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"

	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"
	securityv1alpha1 "github.com/gardener/gardener/pkg/apis/security/v1alpha1"
)

type workloadIdentityValidator struct {
}

// NewWorkloadIdentityValidator returns a new instance of a WorkloadIdentity validator.
func NewWorkloadIdentityValidator() extensionswebhook.Validator {
	return &workloadIdentityValidator{}
}

// Validate checks whether the provider config is empty.
func (wi *workloadIdentityValidator) Validate(_ context.Context, newObj, _ client.Object) error {
	workloadIdentity, ok := newObj.(*securityv1alpha1.WorkloadIdentity)
	if !ok {
		return fmt.Errorf("wrong object type %T", newObj)
	}

	if workloadIdentity.Spec.TargetSystem.ProviderConfig != nil {
		return errors.New("target system provider config must be empty")
	}

	return nil
}
