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

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ListReplicaSets returns the list of ReplicaSets in the given <namespace>.
func (c *Clientset) ListReplicaSets(namespace string, listOptions metav1.ListOptions) (*appsv1.ReplicaSetList, error) {
	replicasets, err := c.Kubernetes().AppsV1().ReplicaSets(namespace).List(listOptions)
	if err != nil {
		return nil, err
	}
	sort.Slice(replicasets.Items, func(i, j int) bool {
		return replicasets.Items[i].ObjectMeta.CreationTimestamp.Before(&replicasets.Items[j].ObjectMeta.CreationTimestamp)
	})
	return replicasets, nil
}

// DeleteReplicaSet deletes a ReplicaSet object.
func (c *Clientset) DeleteReplicaSet(namespace, name string) error {
	return c.Kubernetes().AppsV1().ReplicaSets(namespace).Delete(name, &defaultDeleteOptions)
}
