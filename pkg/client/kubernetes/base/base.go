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
	"k8s.io/client-go/tools/clientcmd"
)

// New returns a new Kubernetes base client.
func New(config *rest.Config, clientset *kubernetes.Clientset, clientConfig clientcmd.ClientConfig) (*Client, error) {
	baseClient := &Client{
		config:          config,
		clientConfig:    clientConfig,
		clientset:       clientset,
		gardenClientset: nil,
		restClient:      clientset.Discovery().RESTClient(),
		resourceAPIGroups: map[string][]string{
			"cronjobs":                  []string{"apis", "batch", "v1beta1"},
			"customresourcedefinitions": []string{"apis", "apiextensions.k8s.io", "v1beta1"},
			"daemonsets":                []string{"apis", "apps", "v1beta2"},
			"deployments":               []string{"apis", "apps", "v1beta2"},
			"ingresses":                 []string{"apis", "extensions", "v1beta1"},
			"jobs":                      []string{"apis", "batch", "v1"},
			"namespaces":                []string{"api", "v1"},
			"persistentvolumeclaims":    []string{"api", "v1"},
			"pods":                   []string{"api", "v1"},
			"replicasets":            []string{"apis", "apps", "v1beta2"},
			"replicationcontrollers": []string{"api", "v1"},
			"services":               []string{"api", "v1"},
			"statefulsets":           []string{"apis", "apps", "v1beta2"},
		},
	}

	gitVersion, err := baseClient.QueryVersion()
	if err != nil {
		return nil, err
	}
	baseClient.version = gitVersion

	return baseClient, nil
}
