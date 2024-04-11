// Copyright 2024 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

// package kubeobjects defines the k8s objects necessary to materialise the GCMx component on the server side
package kubeobjects

import (
	"github.com/Masterminds/semver/v3"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/utils/managedresources"
)

// For all the GCMx elements, of which there is exactly one per GCMx instance (e.g. deployment, service, service account),
// we tend to use the same name. That here is the name.
const gcmxBaseName = "gardener-custom-metrics"

// GetKubeObjectsAsYamlBytes returns the YAML definitions for all k8s objects necessary to materialise the GCMx component.
// In the resulting map, each object is placed under a key which represents its identity in a format appropriate for use
// as key in map-structured k8s objects, such as Secrets and ConfigMaps.
func GetKubeObjectsAsYamlBytes(deploymentName, namespace, image, serverSecretName string, kubernetesVersion *semver.Version) (map[string][]byte, error) {
	registry := managedresources.NewRegistry(kubernetes.ShootScheme, kubernetes.ShootCodec, kubernetes.ShootSerializer)

	return registry.AddAllAndSerialize(
		makeServiceAccount(namespace),
		makeRole(namespace),
		makeRoleBinding(namespace),
		makeClusterRole(),
		makeClusterRoleBinding(namespace),
		makeAuthDelegatorClusterRoleBinding(namespace),
		makeAuthReaderRoleBinding(namespace),
		makeDeployment(deploymentName, namespace, image, serverSecretName),
		makeService(namespace),
		makeAPIService(namespace),
		makePDB(namespace, kubernetesVersion),
		makeVPA(namespace),
	)
}
