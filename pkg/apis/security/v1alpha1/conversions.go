// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	conversion "k8s.io/apimachinery/pkg/conversion"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/gardener/gardener/pkg/apis/security"
)

// Convert_security_TargetSystem_To_v1alpha1_TargetSystem is a manual conversion function.
func Convert_security_TargetSystem_To_v1alpha1_TargetSystem(in *security.TargetSystem, out *TargetSystem, s conversion.Scope) error {
	if err := autoConvert_security_TargetSystem_To_v1alpha1_TargetSystem(in, out, s); err != nil {
		return err
	}

	if in.ProviderConfig == nil {
		out.ProviderConfig = nil
		return nil
	}

	out.ProviderConfig = &runtime.RawExtension{}
	return runtime.Convert_runtime_Object_To_runtime_RawExtension(&in.ProviderConfig, out.ProviderConfig, s)
}

// Convert_v1alpha1_TargetSystem_To_security_TargetSystem is a manual conversion function.
func Convert_v1alpha1_TargetSystem_To_security_TargetSystem(in *TargetSystem, out *security.TargetSystem, s conversion.Scope) error {
	if err := autoConvert_v1alpha1_TargetSystem_To_security_TargetSystem(in, out, s); err != nil {
		return err
	}

	if in.ProviderConfig == nil {
		out.ProviderConfig = nil
		return nil
	}

	return runtime.Convert_runtime_RawExtension_To_runtime_Object(in.ProviderConfig, &out.ProviderConfig, s)
}
