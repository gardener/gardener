// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package networkpolicy

import (
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/gardenlet/controller/federatedseed/networkpolicy/helper"
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
