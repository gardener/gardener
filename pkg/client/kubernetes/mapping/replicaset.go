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
	extensions_v1beta1 "k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ReplicaSet object
type ReplicaSet struct {
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`
	Spec              ReplicaSetSpec   `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`
	Status            ReplicaSetStatus `json:"status,omitempty" protobuf:"bytes,3,opt,name=status"`
}

// ReplicaSetSpec object
type ReplicaSetSpec struct {
	Replicas *int32 `json:"replicas,omitempty" protobuf:"varint,1,opt,name=replicas"`
}

// ReplicaSetStatus object
type ReplicaSetStatus struct {
	Replicas int32 `json:"replicas" protobuf:"varint,1,opt,name=replicas"`
}

// ExtensionsV1beta1ReplicaSet maps a ReplicaSet type from API group extensions/v1beta1 to our type.
func ExtensionsV1beta1ReplicaSet(replicaSet extensions_v1beta1.ReplicaSet) *ReplicaSet {
	return &ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: replicaSet.ObjectMeta.Name,
		},
		Spec: ReplicaSetSpec{
			Replicas: replicaSet.Spec.Replicas,
		},
		Status: ReplicaSetStatus{
			Replicas: replicaSet.Status.Replicas,
		},
	}
}

// AppsV1beta2ReplicaSet maps a ReplicaSet type from API group apps/v1beta2 to our type.
func AppsV1beta2ReplicaSet(replicaSet apps_v1beta2.ReplicaSet) *ReplicaSet {
	return &ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: replicaSet.ObjectMeta.Name,
		},
		Spec: ReplicaSetSpec{
			Replicas: replicaSet.Spec.Replicas,
		},
		Status: ReplicaSetStatus{
			Replicas: replicaSet.Status.Replicas,
		},
	}
}

// AppsV1ReplicaSet maps a ReplicaSet type from API group apps/v1 to our type.
func AppsV1ReplicaSet(replicaSet apps_v1.ReplicaSet) *ReplicaSet {
	return &ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: replicaSet.ObjectMeta.Name,
		},
		Spec: ReplicaSetSpec{
			Replicas: replicaSet.Spec.Replicas,
		},
		Status: ReplicaSetStatus{
			Replicas: replicaSet.Status.Replicas,
		},
	}
}
