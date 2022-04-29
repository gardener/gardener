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

	appsv1 "k8s.io/api/apps/v1"
)

// CheckStatefulSet checks whether the given StatefulSet is healthy.
// A StatefulSet is considered healthy if its controller observed its current revision,
// it is not in an update (i.e. UpdateRevision is empty) and if its current replicas are equal to
// its desired replicas.
func CheckStatefulSet(statefulSet *appsv1.StatefulSet) error {
	if statefulSet.Status.ObservedGeneration < statefulSet.Generation {
		return fmt.Errorf("observed generation outdated (%d/%d)", statefulSet.Status.ObservedGeneration, statefulSet.Generation)
	}

	replicas := int32(1)
	if statefulSet.Spec.Replicas != nil {
		replicas = *statefulSet.Spec.Replicas
	}

	if statefulSet.Status.ReadyReplicas < replicas {
		return fmt.Errorf("not enough ready replicas (%d/%d)", statefulSet.Status.ReadyReplicas, replicas)
	}
	return nil
}

// IsStatefulSetProgressing returns false if the StatefulSet has been fully rolled out. Otherwise, it returns true along
// with a reason, why the StatefulSet is not considered to be fully rolled out.
func IsStatefulSetProgressing(statefulSet *appsv1.StatefulSet) (bool, string) {
	if statefulSet.Status.ObservedGeneration < statefulSet.Generation {
		return true, fmt.Sprintf("observed generation outdated (%d/%d)", statefulSet.Status.ObservedGeneration, statefulSet.Generation)
	}

	desiredReplicas := int32(1)
	if statefulSet.Spec.Replicas != nil {
		desiredReplicas = *statefulSet.Spec.Replicas
	}

	updatedReplicas := statefulSet.Status.UpdatedReplicas
	if updatedReplicas < desiredReplicas {
		return true, fmt.Sprintf("%d of %d replica(s) have been updated", updatedReplicas, desiredReplicas)
	}

	return false, "StatefulSet is fully rolled out"
}
