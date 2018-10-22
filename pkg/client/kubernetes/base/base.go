// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package kubernetesbase

import (
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	apiextensionsclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	apiserviceclientset "k8s.io/kube-aggregator/pkg/client/clientset_generated/clientset"
)

// NewForConfig returns a new Kubernetes base client.
func NewForConfig(config *rest.Config) (*Client, error) {
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	apiregistrationClientset, err := apiserviceclientset.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	apiextensionClientset, err := apiextensionsclientset.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	baseClient := &Client{
		config: config,

		clientset:                clientset,
		apiregistrationClientset: apiregistrationClientset,
		apiextensionClientset:    apiextensionClientset,

		restClient: clientset.Discovery().RESTClient(),

		resourceAPIGroups: map[string][]string{
			CronJobs:                  {"apis", "batch", "v1beta1"},
			CustomResourceDefinitions: {"apis", "apiextensions.k8s.io", "v1beta1"},
			DaemonSets:                {"apis", "apps", "v1"},
			Deployments:               {"apis", "apps", "v1"},
			Ingresses:                 {"apis", "extensions", "v1beta1"},
			Jobs:                      {"apis", "batch", "v1"},
			Namespaces:                {"api", "v1"},
			PersistentVolumeClaims:    {"api", "v1"},
			Pods:                      {"api", "v1"},
			ReplicaSets:               {"apis", "apps", "v1"},
			ReplicationControllers:    {"api", "v1"},
			Services:                  {"api", "v1"},
			StatefulSets:              {"apis", "apps", "v1"},
		},
	}

	gitVersion, err := baseClient.QueryVersion()
	if err != nil {
		return nil, err
	}
	baseClient.version = gitVersion

	return baseClient, nil
}
