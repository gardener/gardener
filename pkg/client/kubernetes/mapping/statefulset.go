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
	apps_v1beta2 "k8s.io/api/apps/v1beta2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// StatefulSet object
type StatefulSet struct {
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`
	Spec              StatefulSetSpec   `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`
	Status            StatefulSetStatus `json:"status,omitempty" protobuf:"bytes,3,opt,name=status"`
}

// StatefulSetSpec object
type StatefulSetSpec struct {
	Replicas *int32 `json:"replicas,omitempty" protobuf:"varint,1,opt,name=replicas"`
}

// StatefulSetStatus object
type StatefulSetStatus struct {
	Replicas        int32 `json:"replicas" protobuf:"varint,2,opt,name=replicas"`
	ReadyReplicas   int32 `json:"readyReplicas,omitempty" protobuf:"varint,3,opt,name=readyReplicas"`
	CurrentReplicas int32 `json:"currentReplicas,omitempty" protobuf:"varint,4,opt,name=currentReplicas"`
	UpdatedReplicas int32 `json:"updatedReplicas,omitempty" protobuf:"varint,5,opt,name=updatedReplicas"`
}

// AppsV1beta2StatefulSet maps a StatefulSet type from API group apps/v1beta2 to our type.
func AppsV1beta2StatefulSet(statefulSet apps_v1beta2.StatefulSet) *StatefulSet {
	return &StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: statefulSet.ObjectMeta.Name,
		},
		Spec: StatefulSetSpec{
			Replicas: statefulSet.Spec.Replicas,
		},
		Status: StatefulSetStatus{
			Replicas:        statefulSet.Status.Replicas,
			ReadyReplicas:   statefulSet.Status.ReadyReplicas,
			CurrentReplicas: statefulSet.Status.CurrentReplicas,
			UpdatedReplicas: statefulSet.Status.UpdatedReplicas,
		},
	}
}

// AppsV1StatefulSet maps a StatefulSet type from API group apps/v1 to our type.
func AppsV1StatefulSet(statefulSet apps_v1.StatefulSet) *StatefulSet {
	return &StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: statefulSet.ObjectMeta.Name,
		},
		Spec: StatefulSetSpec{
			Replicas: statefulSet.Spec.Replicas,
		},
		Status: StatefulSetStatus{
			Replicas:        statefulSet.Status.Replicas,
			ReadyReplicas:   statefulSet.Status.ReadyReplicas,
			CurrentReplicas: statefulSet.Status.CurrentReplicas,
			UpdatedReplicas: statefulSet.Status.UpdatedReplicas,
		},
	}
}
