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

package kubernetesv18

import (
	"github.com/gardener/gardener/pkg/client/kubernetes/mapping"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ListRoleBindings returns a list of rolebindings in a given <namespace>.
// The selection can be restricted by passsing an <selector>.
func (c *Client) ListRoleBindings(namespace string, selector metav1.ListOptions) ([]*mapping.RoleBinding, error) {
	roleBindings, err := c.
		Clientset.
		RbacV1().
		RoleBindings(namespace).
		List(selector)
	if err != nil {
		return nil, err
	}
	roleBindingList := make([]*mapping.RoleBinding, len(roleBindings.Items))
	for i, rb := range roleBindings.Items {
		roleBindingList[i] = mapping.RbacV1RoleBinding(rb)
	}
	return roleBindingList, nil
}
