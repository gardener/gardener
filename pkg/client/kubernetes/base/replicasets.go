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
	"sort"

	"github.com/gardener/gardener/pkg/client/kubernetes/mapping"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var replicasetPath = []string{"apis", "extensions", "v1beta1", "replicasets"}

// ListReplicaSets returns the list of ReplicaSets in the given <namespace>.
func (c *Client) ListReplicaSets(namespace string, listOptions metav1.ListOptions) ([]*mapping.ReplicaSet, error) {
	var replicasetList []*mapping.ReplicaSet
	replicasets, err := c.
		Clientset.
		ExtensionsV1beta1().
		ReplicaSets(namespace).
		List(listOptions)
	if err != nil {
		return nil, err
	}
	sort.Slice(replicasets.Items, func(i, j int) bool {
		return replicasets.Items[i].ObjectMeta.CreationTimestamp.Before(&replicasets.Items[j].ObjectMeta.CreationTimestamp)
	})
	for _, replicaset := range replicasets.Items {
		replicasetList = append(replicasetList, mapping.ExtensionsV1beta1ReplicaSet(replicaset))
	}
	return replicasetList, nil
}

// DeleteReplicaSet deletes a ReplicaSet object.
func (c *Client) DeleteReplicaSet(namespace, name string) error {
	return c.
		Clientset.
		ExtensionsV1beta1().
		ReplicaSets(namespace).
		Delete(name, &defaultDeleteOptions)
}

// CleanupReplicaSets deletes all the ReplicaSets in the cluster other than those stored in the
// exceptions map <exceptions>.
func (c *Client) CleanupReplicaSets(exceptions map[string]bool) error {
	return c.CleanupResource(exceptions, true, replicasetPath...)
}

// CheckReplicaSetCleanup will check whether all the ReplicaSets in the cluster other than those
// stored in the exceptions map <exceptions> have been deleted. It will return an error
// in case it has not finished yet, and nil if all resources are gone.
func (c *Client) CheckReplicaSetCleanup(exceptions map[string]bool) (bool, error) {
	return c.CheckResourceCleanup(exceptions, true, replicasetPath...)
}
