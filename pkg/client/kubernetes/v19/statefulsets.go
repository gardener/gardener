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

package kubernetesv19

var statefulsetPath = []string{"apis", "apps", "v1", "statefulsets"}

// DeleteStatefulSet deletes a StatefulSet object.
func (c *Client) DeleteStatefulSet(namespace, name string) error {
	return c.Clientset.AppsV1().StatefulSets(namespace).Delete(name, &defaultDeleteOptions)
}

// CleanupStatefulSets deletes all the StatefulSets in the Shoot cluster other than those stored in the
// exceptions map <exceptions>.
func (c *Client) CleanupStatefulSets(exceptions map[string]bool) error {
	return c.CleanupResource(exceptions, true, statefulsetPath...)
}

// CheckStatefulSetCleanup will check whether all the StatefulSets in the Shoot cluster other than those
// stored in the exceptions map <exceptions> have been deleted. It will return an error
// in case it has not finished yet, and nil if all resources are gone.
func (c *Client) CheckStatefulSetCleanup(exceptions map[string]bool) (bool, error) {
	return c.CheckResourceCleanup(exceptions, true, statefulsetPath...)
}
