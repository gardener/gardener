// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validator

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"sigs.k8s.io/controller-runtime/pkg/client"

	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"
)

type secretValidator struct{}

// NewSecretValidator returns a new instance of a secret validator.
func NewSecretValidator() extensionswebhook.Validator {
	return &secretValidator{}
}

// Validate checks whether the data is empty.
func (s *secretValidator) Validate(_ context.Context, newObj, oldObj client.Object) error {
	secret, ok := newObj.(*corev1.Secret)
	if !ok {
		return fmt.Errorf("wrong object type %T", newObj)
	}

	if oldObj != nil {
		oldSecret, ok := oldObj.(*corev1.Secret)
		if !ok {
			return fmt.Errorf("wrong object type %T for old object", oldObj)
		}

		if apiequality.Semantic.DeepEqual(secret.Data, oldSecret.Data) {
			return nil
		}
	}

	if len(secret.Data) != 0 {
		return fmt.Errorf("secret data should be empty")
	}

	return nil
}
