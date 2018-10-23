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

package kubernetesv112

import (
	"github.com/gardener/gardener/pkg/client/kubernetes/base"
	"github.com/gardener/gardener/pkg/client/kubernetes/v111"
	"k8s.io/client-go/rest"
)

// NewForConfig returns a new Kubernetes v1.12 client.
func NewForConfig(config *rest.Config) (*Client, error) {
	v111Client, err := kubernetesv111.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return NewFrom(v111Client), nil
}

// NewFrom creates a new client from the given kubernetesv111.Client.
func NewFrom(v111Client *kubernetesv111.Client) *Client {
	return &Client{
		Client: v111Client,
	}
}

// NewFromBase creates a new client from the given kubernetesbase.Client.
func NewFromBase(client *kubernetesbase.Client) *Client {
	return NewFrom(kubernetesv111.NewFromBase(client))
}
