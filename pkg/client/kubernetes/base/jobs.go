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
	batch_v1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var jobPath = []string{"apis", "batch", "v1", "jobs"}

// GetJob returns a Job object.
func (c *Client) GetJob(namespace, name string) (*batch_v1.Job, error) {
	return c.
		Clientset.
		BatchV1().
		Jobs(namespace).
		Get(name, metav1.GetOptions{})
}

// DeleteJob deletes a Job object.
func (c *Client) DeleteJob(namespace, name string) error {
	return c.
		Clientset.
		BatchV1().
		Jobs(namespace).
		Delete(name, &defaultDeleteOptions)
}

// CleanupJobs deletes all the Jobs in the cluster other than those stored in the
// exceptions map <exceptions>.
func (c *Client) CleanupJobs(exceptions map[string]bool) error {
	return c.CleanupResource(exceptions, true, jobPath...)
}

// CheckJobCleanup will check whether all the Jobs in the cluster other than those
// stored in the exceptions map <exceptions> have been deleted. It will return an error
// in case it has not finished yet, and nil if all resources are gone.
func (c *Client) CheckJobCleanup(exceptions map[string]bool) (bool, error) {
	return c.CheckResourceCleanup(exceptions, true, jobPath...)
}
