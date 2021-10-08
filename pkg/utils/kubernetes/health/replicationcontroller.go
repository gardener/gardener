// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package health

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
)

// CheckReplicationController checks whether the given ReplicationController is healthy.
// A ReplicationController is considered healthy if the controller observed its current revision and
// if the number of ready replicas is equal to the number of replicas.
func CheckReplicationController(rc *corev1.ReplicationController) error {
	if rc.Status.ObservedGeneration < rc.Generation {
		return fmt.Errorf("observed generation outdated (%d/%d)", rc.Status.ObservedGeneration, rc.Generation)
	}

	replicas := rc.Spec.Replicas
	if replicas != nil && rc.Status.ReadyReplicas < *replicas {
		return fmt.Errorf("ReplicationController does not have minimum availability")
	}

	return nil
}
