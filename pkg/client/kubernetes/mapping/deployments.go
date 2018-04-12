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

package mapping

import (
	apps_v1 "k8s.io/api/apps/v1"
	apps_v1beta1 "k8s.io/api/apps/v1beta1"
	apps_v1beta2 "k8s.io/api/apps/v1beta2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Deployment object
type Deployment struct {
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`
	Spec              DeploymentSpec   `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`
	Status            DeploymentStatus `json:"status,omitempty" protobuf:"bytes,3,opt,name=status"`
}

// DeploymentSpec object
type DeploymentSpec struct {
	Replicas *int32 `json:"replicas,omitempty" protobuf:"varint,1,opt,name=replicas"`
}

// DeploymentStatus object
type DeploymentStatus struct {
	AvailableReplicas int32 `json:"availableReplicas,omitempty" protobuf:"varint,4,opt,name=availableReplicas"`
}

// AppsV1beta1Deployment maps a Deployment type from API group apps/v1beta1 to our type.
func AppsV1beta1Deployment(deployment apps_v1beta1.Deployment) *Deployment {
	return &Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: deployment.ObjectMeta.Name,
		},
		Spec: DeploymentSpec{
			Replicas: deployment.Spec.Replicas,
		},
		Status: DeploymentStatus{
			AvailableReplicas: deployment.Status.AvailableReplicas,
		},
	}
}

// AppsV1beta2Deployment maps a Deployment type from API group apps/v1beta2 to our type.
func AppsV1beta2Deployment(deployment apps_v1beta2.Deployment) *Deployment {
	return &Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: deployment.ObjectMeta.Name,
		},
		Spec: DeploymentSpec{
			Replicas: deployment.Spec.Replicas,
		},
		Status: DeploymentStatus{
			AvailableReplicas: deployment.Status.AvailableReplicas,
		},
	}
}

// AppsV1Deployment maps a Deployment type from API group apps/v1 to our type.
func AppsV1Deployment(deployment apps_v1.Deployment) *Deployment {
	return &Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: deployment.ObjectMeta.Name,
		},
		Spec: DeploymentSpec{
			Replicas: deployment.Spec.Replicas,
		},
		Status: DeploymentStatus{
			AvailableReplicas: deployment.Status.AvailableReplicas,
		},
	}
}
