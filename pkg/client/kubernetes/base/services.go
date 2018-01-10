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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var servicePath = []string{"api", "v1", "services"}

// GetService returns the desired Service object.
func (c *Client) GetService(namespace, name string) (*corev1.Service, error) {
	return c.
		Clientset.
		CoreV1().
		Services(namespace).
		Get(name, metav1.GetOptions{})
}

// CleanupServices deletes all the Services in the cluster other than those stored in the
// exceptions map <exceptions>.
func (c *Client) CleanupServices(exceptions map[string]bool) error {
	return c.CleanupResource(exceptions, true, servicePath...)
}

// CheckServiceCleanup will check whether all the Services in the cluster other than those
// stored in the exceptions map <exceptions> have been deleted. It will return an error
// in case it has not finished yet, and nil if all resources are gone.
func (c *Client) CheckServiceCleanup(exceptions map[string]bool) (bool, error) {
	return c.CheckResourceCleanup(exceptions, true, servicePath...)
}
