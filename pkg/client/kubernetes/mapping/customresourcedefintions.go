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
	apiextensions_v1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CustomResourceDefinition object
type CustomResourceDefinition struct {
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`
	Spec              CustomResourceDefinitionSpec   `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`
	Status            CustomResourceDefinitionStatus `json:"status,omitempty" protobuf:"bytes,3,opt,name=status"`
}

// CustomResourceDefinitionSpec object
type CustomResourceDefinitionSpec struct{}

// CustomResourceDefinitionStatus object
type CustomResourceDefinitionStatus struct {
	Conditions []CustomResourceDefinitionCondition `json:"conditions" protobuf:"bytes,1,opt,name=conditions"`
}

// CustomResourceDefinitionCondition object
type CustomResourceDefinitionCondition struct {
	Type               string      `json:"type" protobuf:"bytes,1,opt,name=type,casttype=CustomResourceDefinitionConditionType"`
	Status             string      `json:"status" protobuf:"bytes,2,opt,name=status,casttype=ConditionStatus"`
	LastTransitionTime metav1.Time `json:"lastTransitionTime,omitempty" protobuf:"bytes,3,opt,name=lastTransitionTime"`
	Reason             string      `json:"reason,omitempty" protobuf:"bytes,4,opt,name=reason"`
	Message            string      `json:"message,omitempty" protobuf:"bytes,5,opt,name=message"`
}

// ApiextensionsV1beta1CustomResourceDefinition maps a CustomResourceDefinition type from API group
// apiextensions.k8s.io/v1beta1 to our type.
func ApiextensionsV1beta1CustomResourceDefinition(crd apiextensions_v1beta1.CustomResourceDefinition) *CustomResourceDefinition {
	var conditions []CustomResourceDefinitionCondition
	for _, condition := range crd.Status.Conditions {
		conditions = append(conditions, CustomResourceDefinitionCondition{
			Type:               string(condition.Type),
			Status:             string(condition.Status),
			LastTransitionTime: condition.LastTransitionTime,
			Reason:             condition.Reason,
			Message:            condition.Message,
		})
	}
	return &CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name: crd.ObjectMeta.Name,
		},
		Status: CustomResourceDefinitionStatus{
			Conditions: conditions,
		},
	}
}
