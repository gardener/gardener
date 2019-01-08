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
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CreateConfigMap creates a new ConfigMap object.
func (c *Clientset) CreateConfigMap(namespace, name string, data map[string]string, updateIfExists bool) (*corev1.ConfigMap, error) {
	secret, err := c.kubernetes.CoreV1().ConfigMaps(namespace).Create(&corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Data: data,
	})
	if err != nil && apierrors.IsAlreadyExists(err) && updateIfExists {
		return c.UpdateConfigMap(namespace, name, data)
	}
	return secret, err
}

// UpdateConfigMap updates an already existing ConfigMap object.
func (c *Clientset) UpdateConfigMap(namespace, name string, data map[string]string) (*corev1.ConfigMap, error) {
	return c.kubernetes.CoreV1().ConfigMaps(namespace).Update(&corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Data: data,
	})
}

// GetConfigMap returns a ConfigMap object.
func (c *Clientset) GetConfigMap(namespace, name string) (*corev1.ConfigMap, error) {
	return c.kubernetes.CoreV1().ConfigMaps(namespace).Get(name, metav1.GetOptions{})
}

// DeleteConfigMap deletes a ConfigMap object.
func (c *Clientset) DeleteConfigMap(namespace, name string) error {
	return c.kubernetes.CoreV1().ConfigMaps(namespace).Delete(name, &defaultDeleteOptions)
}
