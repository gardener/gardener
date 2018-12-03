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
	batch_v1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GetJob returns a Job object.
func (c *Clientset) GetJob(namespace, name string) (*batch_v1.Job, error) {
	return c.kubernetes.BatchV1().Jobs(namespace).Get(name, metav1.GetOptions{})
}

// DeleteJob deletes a Job object.
func (c *Clientset) DeleteJob(namespace, name string) error {
	return c.kubernetes.BatchV1().Jobs(namespace).Delete(name, &defaultDeleteOptions)
}
