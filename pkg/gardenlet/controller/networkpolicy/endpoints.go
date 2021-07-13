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

package networkpolicy

import (
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/gardenlet/controller/networkpolicy/helper"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"k8s.io/client-go/tools/cache"
)

func (c *Controller) endpointAdd(_ interface{}) {
	c.enqueueNamespaces()
}

func (c *Controller) endpointUpdate(_, _ interface{}) {
	c.enqueueNamespaces()
}

func (c *Controller) endpointDelete(_ interface{}) {
	c.enqueueNamespaces()
}

func (c *Controller) enqueueNamespaces() {
	namespaces := &corev1.NamespaceList{}
	if err := c.seedClient.List(c.ctx, namespaces, &client.ListOptions{
		LabelSelector: c.shootNamespaceSelector,
	}); err != nil {
		c.log.Errorf("Failed to enqueue namespaces to update NetworkPolicy %q - unable to list Shoot namespaces: %v", helper.AllowToSeedAPIServer, err)
		return
	}

	for _, namespace := range namespaces.Items {
		key, err := cache.MetaNamespaceKeyFunc(&namespace)
		if err != nil {
			c.log.Errorf("Failed to enqueue namespaces to update NetworkPolicy %q for namespace %q - couldn't get key for namespace: %v", helper.AllowToSeedAPIServer, namespace.Name, err)
			continue
		}
		c.namespaceQueue.Add(key)
	}
	c.namespaceQueue.Add(v1beta1constants.GardenNamespace)
}
