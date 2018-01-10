// Copyright 2018 The Gardener Authors.
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

package kubernetesv18

import (
	kubernetesv17 "github.com/gardener/gardener/pkg/client/kubernetes/v17"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// New returns a new Kubernetes v1.8 client.
func New(config *rest.Config, clientset *kubernetes.Clientset, clientConfig clientcmd.ClientConfig) (*Client, error) {
	v17Client, err := kubernetesv17.New(config, clientset, clientConfig)
	if err != nil {
		return nil, err
	}
	return &Client{
		Client: v17Client,
	}, nil
}
