// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package index

import (
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// SecretRefNamespaceField is the field name for the index function that extracts the corresponding field from SecretBinding.
const SecretRefNamespaceField string = "secretRef.namespace"

// SecretRefNamespaceIndexerFunc extracts the secretRef.namespace field of a SecretBinding.
func SecretRefNamespaceIndexerFunc(rawObj client.Object) []string {
	secretBinding, ok := rawObj.(*gardencorev1beta1.SecretBinding)
	if !ok {
		return []string{}
	}
	return []string{secretBinding.SecretRef.Namespace}
}

// SecretBindingNameField is the field name for the index function that extracts the corresponding field from Shoot.
const SecretBindingNameField string = "spec.secretBindingName"

// SecretBindingNameIndexerFunc extracts the spec.secretBindingName field of a Shoot.
func SecretBindingNameIndexerFunc(rawObj client.Object) []string {
	shoot, ok := rawObj.(*gardencorev1beta1.Shoot)
	if !ok {
		return []string{}
	}
	return []string{shoot.Spec.SecretBindingName}
}
