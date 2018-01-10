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

// GetConfigMap returns a ConfigMap object.
func (c *Client) GetConfigMap(namespace, name string) (*corev1.ConfigMap, error) {
	return c.
		Clientset.
		CoreV1().
		ConfigMaps(namespace).
		Get(name, metav1.GetOptions{})
}

// DeleteConfigMap deletes a ConfigMap object.
func (c *Client) DeleteConfigMap(namespace, name string) error {
	return c.
		Clientset.
		CoreV1().
		ConfigMaps(namespace).
		Delete(name, &defaultDeleteOptions)
}
