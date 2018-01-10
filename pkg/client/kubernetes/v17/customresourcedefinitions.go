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

package kubernetesv17

var (
	tprPath = []string{"apis", "extensions", "v1beta1", "thirdpartyresources"}
	crdPath = []string{"apis", "apiextensions.k8s.io", "v1beta1", "customresourcedefinitions"}
)

// CleanupCRDs deletes all the TPRs/CRDs in the cluster other than those stored in the
// exceptions map <exceptions>.
func (c *Client) CleanupCRDs(exceptions map[string]bool) error {
	err := c.CleanupResource(exceptions, false, tprPath...)
	if err != nil {
		return err
	}
	return c.CleanupResource(exceptions, false, crdPath...)
}

// CheckCRDCleanup will check whether all the CRDs in the cluster other than those
// stored in the exceptions map <exceptions> have been deleted. It will return an error
// in case it has not finished yet, and nil if all resources are gone.
func (c *Client) CheckCRDCleanup(exceptions map[string]bool) (bool, error) {
	finished, err := c.CheckResourceCleanup(exceptions, false, tprPath...)
	if err != nil || !finished {
		return finished, err
	}
	return c.CheckResourceCleanup(exceptions, false, crdPath...)
}
