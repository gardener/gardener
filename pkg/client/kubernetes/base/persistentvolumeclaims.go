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

var persistentvolumeclaimPath = []string{"api", "v1", "persistentvolumeclaims"}

// CleanupPersistentVolumeClaims deletes all the PersistentVolumeClaims in the cluster other than those stored in the
// exceptions map <exceptions>.
func (c *Client) CleanupPersistentVolumeClaims(exceptions map[string]bool) error {
	return c.CleanupResource(exceptions, true, persistentvolumeclaimPath...)
}

// CheckPersistentVolumeClaimCleanup will check whether all the PersistentVolumeClaims in the
// cluster other than those stored in the exceptions map <exceptions> have been deleted. It will
// return an error in case it has not finished yet, and nil if all resources are gone.
func (c *Client) CheckPersistentVolumeClaimCleanup(exceptions map[string]bool) (bool, error) {
	return c.CheckResourceCleanup(exceptions, true, persistentvolumeclaimPath...)
}
