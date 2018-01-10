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

package mapping

import (
	rbac_v1 "k8s.io/api/rbac/v1"
	rbac_v1beta1 "k8s.io/api/rbac/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// RoleBinding object
type RoleBinding struct {
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`
	Subjects          []RoleBindingSubject `json:"subjects,omitempty" protobuf:"bytes,2,rep,name=subjects"`
}

// RoleBindingSubject object
type RoleBindingSubject struct {
	Kind string `json:"kind,omitempty" protobuf:"bytes,1,opt,name=kind"`
	Name string `json:"name,omitempty" protobuf:"bytes,1,opt,name=name"`
}

// RbacV1Beta1RoleBinding maps a RoleBinding type from API group rbac/v1beta1 to our type.
func RbacV1Beta1RoleBinding(rb rbac_v1beta1.RoleBinding) *RoleBinding {
	subjectList := make([]RoleBindingSubject, len(rb.Subjects))
	for i, subject := range rb.Subjects {
		mappedSubject := RoleBindingSubject{
			Name: subject.Name,
			Kind: subject.Kind,
		}
		subjectList[i] = mappedSubject
	}
	return &RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: rb.ObjectMeta.Name,
		},
		Subjects: subjectList,
	}
}

// RbacV1RoleBinding maps a RoleBinding type from API group rbac/v1 to our type.
func RbacV1RoleBinding(rb rbac_v1.RoleBinding) *RoleBinding {
	subjectList := make([]RoleBindingSubject, len(rb.Subjects))
	for i, subject := range rb.Subjects {
		mappedSubject := RoleBindingSubject{
			Name: subject.Name,
			Kind: subject.Kind,
		}
		subjectList[i] = mappedSubject
	}
	return &RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: rb.ObjectMeta.Name,
		},
		Subjects: subjectList,
	}
}
