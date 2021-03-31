// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors.
//
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/pointer"
)

func addDefaultingFuncs(scheme *runtime.Scheme) error {
	return RegisterDefaults(scheme)
}

// SetDefaults_Blueprint sets default values for blueprint objects
func SetDefaults_Blueprint(obj *Blueprint) {
	if len(obj.JSONSchemaVersion) == 0 {
		obj.JSONSchemaVersion = "https://json-schema.org/draft/2019-09/schema"
	}

	for i := range obj.Imports {
		SetDefaults_DefinitionImport(&(obj.Imports[i]))
	}
}

// SetDefaults_DefinitionImport sets default values for the ImportDefinition objects
func SetDefaults_DefinitionImport(obj *ImportDefinition) {
	if obj.Required == nil {
		obj.Required = pointer.BoolPtr(true)
	}
}

// SetDefaults_Installation sets default values for installation objects
func SetDefaults_Installation(obj *Installation) {
	// default the namespace of imports
	for i, dataImport := range obj.Spec.Imports.Data {
		if dataImport.ConfigMapRef != nil {
			if len(dataImport.ConfigMapRef.Namespace) == 0 {
				obj.Spec.Imports.Data[i].ConfigMapRef.Namespace = obj.GetNamespace()
			}
		}
		if dataImport.SecretRef != nil {
			if len(dataImport.SecretRef.Namespace) == 0 {
				obj.Spec.Imports.Data[i].SecretRef.Namespace = obj.GetNamespace()
			}
		}
	}
}
