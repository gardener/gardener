// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

//nolint:revive
package v1alpha1

import (
	"fmt"

	"k8s.io/apimachinery/pkg/conversion"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/gardener/gardener/pkg/apis/operations"
)

func addConversionFuncs(scheme *runtime.Scheme) error {
	if err := scheme.AddFieldLabelConversionFunc(SchemeGroupVersion.WithKind("Bastion"),
		func(label, value string) (string, string, error) {
			switch label {
			case "metadata.name", "metadata.namespace", operations.BastionSeedName, operations.BastionShootName:
				return label, value, nil
			default:
				return "", "", fmt.Errorf("field label not supported: %s", label)
			}
		},
	); err != nil {
		return err
	}

	// Add non-generated conversion functions

	if err := scheme.AddConversionFunc((*Bastion)(nil), (*operations.Bastion)(nil), func(a, b any, scope conversion.Scope) error {
		return Convert_v1alpha1_Bastion_To_operations_Bastion(a.(*Bastion), b.(*operations.Bastion), scope)
	}); err != nil {
		return err
	}

	if err := scheme.AddConversionFunc((*BastionSpec)(nil), (*operations.BastionSpec)(nil), func(a, b any, scope conversion.Scope) error {
		return Convert_v1alpha1_BastionSpec_To_operations_BastionSpec(a.(*BastionSpec), b.(*operations.BastionSpec), scope)
	}); err != nil {
		return err
	}

	if err := scheme.AddConversionFunc((*operations.Bastion)(nil), (*Bastion)(nil), func(a, b any, scope conversion.Scope) error {
		return Convert_operations_Bastion_To_v1alpha1_Bastion(a.(*operations.Bastion), b.(*Bastion), scope)
	}); err != nil {
		return err
	}

	if err := scheme.AddConversionFunc((*operations.BastionSpec)(nil), (*BastionSpec)(nil), func(a, b any, scope conversion.Scope) error {
		return Convert_operations_BastionSpec_To_v1alpha1_BastionSpec(a.(*operations.BastionSpec), b.(*BastionSpec), scope)
	}); err != nil {
		return err
	}

	return nil
}
