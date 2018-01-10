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

package kubernetesbase

import (
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var namespacePath = []string{"api", "v1", "namespaces"}

// CreateNamespace creates a new Namespace object.
func (c *Client) CreateNamespace(name string, updateIfExists bool) (*corev1.Namespace, error) {
	namespace, err := c.
		Clientset.
		CoreV1().
		Namespaces().
		Create(&corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
			},
		})
	if err != nil && apierrors.IsAlreadyExists(err) && updateIfExists {
		return c.UpdateNamespace(name)
	}
	return namespace, err
}

// UpdateNamespace updates an already existing Namespace object.
func (c *Client) UpdateNamespace(name string) (*corev1.Namespace, error) {
	return c.
		Clientset.
		CoreV1().
		Namespaces().
		Update(&corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
			},
		})
}

// GetNamespace returns a Namespace object.
func (c *Client) GetNamespace(name string) (*corev1.Namespace, error) {
	return c.
		Clientset.
		CoreV1().
		Namespaces().
		Get(name, metav1.GetOptions{})
}

// ListNamespaces returns a list of namespaces. The selection can be restricted by passing a <selector>.
func (c *Client) ListNamespaces(selector metav1.ListOptions) (*corev1.NamespaceList, error) {
	return c.
		Clientset.
		CoreV1().
		Namespaces().
		List(selector)
}

// DeleteNamespace deletes a namespace.
func (c *Client) DeleteNamespace(name string) error {
	deleteGracePeriod := int64(1)
	return c.
		Clientset.
		CoreV1().
		Namespaces().
		Delete(name, &metav1.DeleteOptions{
			PropagationPolicy:  &propagationPolicy,
			GracePeriodSeconds: &deleteGracePeriod,
		})
}

// CleanupNamespaces deletes all the Namespaces in the cluster other than those stored in the
// exceptions map <exceptions>.
func (c *Client) CleanupNamespaces(exceptions map[string]bool) error {
	return c.CleanupResource(exceptions, false, namespacePath...)
}

// CheckNamespaceCleanup will check whether all the Namespaces in the cluster other than those
// stored in the exceptions map <exceptions> have been deleted. It will return an error
// in case it has not finished yet, and nil if all resources are gone.
func (c *Client) CheckNamespaceCleanup(exceptions map[string]bool) (bool, error) {
	return c.CheckResourceCleanup(exceptions, false, namespacePath...)
}
