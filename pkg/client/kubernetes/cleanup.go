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

package kubernetes

import (
	"errors"
	"fmt"

	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

// ListResources will return a list of Kubernetes resources as JSON byte slice.
func (c *Clientset) ListResources(absPath ...string) (unstructured.Unstructured, error) {
	var resources unstructured.Unstructured
	if err := c.restClient.Get().AbsPath(absPath...).Do().Into(&resources); err != nil {
		return unstructured.Unstructured{}, err
	}
	return resources, nil
}

// CleanupResources will delete all resources except for those stored in the <exceptions> map.
func (c *Clientset) CleanupResources(exceptions map[string]map[string]bool, overwriteResources map[string][]string) error {
	resources := c.resourceAPIGroups
	if overwriteResources != nil {
		resources = overwriteResources
	}

	for resource, apiGroupPath := range resources {
		if resource == CustomResourceDefinitions {
			continue
		}
		if err := c.CleanupAPIGroupResources(exceptions, resource, apiGroupPath); err != nil {
			return err
		}
	}
	return nil
}

// CleanupAPIGroupResources will clean up all resources of a single API group.
func (c *Clientset) CleanupAPIGroupResources(exceptions map[string]map[string]bool, resource string, apiGroupPath []string) error {
	resourceList, err := c.ListResources(append(apiGroupPath, resource)...)
	if err != nil {
		return err
	}

	return resourceList.EachListItem(func(o runtime.Object) error {
		var (
			item            = o.(*unstructured.Unstructured)
			namespace       = item.GetNamespace()
			name            = item.GetName()
			ownerReferences = item.GetOwnerReferences()
			absPathDelete   = buildResourcePath(apiGroupPath, resource, namespace, name)
		)

		if mustOmitResource(exceptions, resource, namespace, name, ownerReferences) {
			return nil
		}

		return c.restClient.Delete().AbsPath(absPathDelete...).Do().Error()
	})
}

// CheckResourceCleanup will check whether all resources except for those in the <exceptions> map have been deleted.
func (c *Clientset) CheckResourceCleanup(logger *logrus.Entry, exceptions map[string]map[string]bool, resource string, apiGroupPath []string) (bool, error) {
	resourceList, err := c.ListResources(append(apiGroupPath, resource)...)
	if err != nil {
		return false, err
	}

	if err := resourceList.EachListItem(func(o runtime.Object) error {
		var (
			item            = o.(*unstructured.Unstructured)
			name            = item.GetName()
			namespace       = item.GetNamespace()
			ownerReferences = item.GetOwnerReferences()
		)

		if mustOmitResource(exceptions, resource, namespace, name, ownerReferences) {
			return nil
		}

		message := fmt.Sprintf("Waiting for '%s' (resource '%s') to be deleted", name, resource)
		logger.Info(message)

		return errors.New(message)
	}); err != nil {
		return false, nil
	}
	return true, nil
}

func buildResourcePath(apiGroupPath []string, resource, namespace, name string) []string {
	if len(namespace) > 0 {
		apiGroupPath = append(apiGroupPath, "namespaces", namespace)
	}
	return append(apiGroupPath, resource, name)
}

func mustOmitResource(exceptionMap map[string]map[string]bool, resource, namespace, name string, ownerReferences []metav1.OwnerReference) bool {
	// Skip resources with owner references and rely on Kubernetes garbage collection to clean them up.
	if len(ownerReferences) > 0 {
		return true
	}

	if exceptions, ok := exceptionMap[resource]; ok {
		id := name
		if len(namespace) > 0 {
			id = fmt.Sprintf("%s/%s", namespace, name)
		}
		if omit, ok := exceptions[id]; ok {
			return omit
		}
		return false
	}
	return false
}
