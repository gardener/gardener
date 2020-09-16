// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package networkpolicy

import (
	"k8s.io/client-go/tools/cache"
)

func (c *Controller) namespaceAdd(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		c.log.Errorf("Couldn't get key for object %+v: %v", obj, err)
		return
	}
	c.namespaceQueue.Add(key)
}

func (c *Controller) namespaceUpdate(_, newObj interface{}) {
	c.namespaceAdd(newObj)
}
