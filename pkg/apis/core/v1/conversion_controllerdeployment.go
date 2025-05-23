// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package v1

import (
	"encoding/json"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/conversion"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/gardener/gardener/pkg/apis/core"
)

const (
	// MigrationControllerDeploymentType is an annotation added to ControllerDeployment resources in v1 if the v1beta1
	// version used a custom type (not built-in). The value contains the value of the type field.
	MigrationControllerDeploymentType = "migration.controllerdeployment.gardener.cloud/type"
	// MigrationControllerDeploymentProviderConfig is an annotation added to ControllerDeployment resources in v1 if the
	// v1beta1 version used a custom type (not built-in). The value contains the value of the providerConfig field.
	MigrationControllerDeploymentProviderConfig = "migration.controllerdeployment.gardener.cloud/providerConfig"
)

func Convert_v1_ControllerDeployment_To_core_ControllerDeployment(in *ControllerDeployment, out *core.ControllerDeployment, s conversion.Scope) error {
	if err := autoConvert_v1_ControllerDeployment_To_core_ControllerDeployment(in, out, s); err != nil {
		return err
	}

	if deploymentType, ok := in.Annotations[MigrationControllerDeploymentType]; ok {
		out.Type = deploymentType
		delete(in.Annotations, MigrationControllerDeploymentType)
	}

	if providerConfig, ok := in.Annotations[MigrationControllerDeploymentProviderConfig]; ok {
		out.ProviderConfig = &runtime.Unknown{
			ContentType: runtime.ContentTypeJSON,
			Raw:         []byte(providerConfig),
		}
		delete(in.Annotations, MigrationControllerDeploymentProviderConfig)
	}

	return nil
}

func Convert_core_ControllerDeployment_To_v1_ControllerDeployment(in *core.ControllerDeployment, out *ControllerDeployment, s conversion.Scope) error {
	if err := autoConvert_core_ControllerDeployment_To_v1_ControllerDeployment(in, out, s); err != nil {
		return err
	}

	if len(in.Type) > 0 {
		metav1.SetMetaDataAnnotation(&out.ObjectMeta, MigrationControllerDeploymentType, in.Type)
	}

	if in.ProviderConfig != nil {
		providerConfigBytes, err := json.Marshal(in.ProviderConfig)
		if err != nil {
			return err
		}

		metav1.SetMetaDataAnnotation(&out.ObjectMeta, MigrationControllerDeploymentProviderConfig, string(providerConfigBytes))
	}

	return nil
}
