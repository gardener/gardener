// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package project

import (
	"strings"

	"k8s.io/client-go/tools/cache"

	"github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/pkg/operation/common"
)

func (c *Controller) roleBindingUpdate(old, new interface{}) {
	c.roleBindingDelete(new)
}

func (c *Controller) roleBindingDelete(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		logger.Logger.Errorf("Couldn't get key for object %+v: %v", obj, err)
		return
	}

	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		logger.Logger.Errorf("Couldn't get key for object %+v: %v", obj, err)
		return
	}

	if name == "gardener.cloud:system:project-member" ||
		name == "gardener.cloud:system:project-viewer" ||
		strings.HasPrefix(name, "gardener.cloud:extension:project:") {

		logger.Logger.Debugf("[PROJECT RECONCILE] %q rolebinding modified", key)

		project, err := common.ProjectForNamespace(c.projectLister, namespace)
		if err != nil {
			logger.Logger.Errorf("Couldn't get list keys for object %+v: %v", obj, err)
			return
		}

		if project.DeletionTimestamp == nil {
			c.projectQueue.Add(project.Name)
		}
	}
}
