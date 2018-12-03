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
	"sort"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ListNodes returns a list of Nodes.
func (c *Clientset) ListNodes(listOptions metav1.ListOptions) (*corev1.NodeList, error) {
	nodes, err := c.kubernetes.CoreV1().Nodes().List(listOptions)
	if err != nil {
		return nil, err
	}
	sort.Slice(nodes.Items, func(i, j int) bool {
		return nodes.Items[i].ObjectMeta.CreationTimestamp.Before(&nodes.Items[j].ObjectMeta.CreationTimestamp)
	})
	return nodes, nil
}
