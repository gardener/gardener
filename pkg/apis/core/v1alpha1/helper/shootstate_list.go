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

package helper

import (
	gardencorev1alpha1 "github.com/gardener/gardener/pkg/apis/core/v1alpha1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
)

// ExtensionResourceStateList is a list of ExtensionResourceStates
type ExtensionResourceStateList []gardencorev1alpha1.ExtensionResourceState

// Get retrieves an ExtensionResourceState for given kind, name and purpose from a list of ExtensionResourceStates
// If no ExtensionResourceStates can be found, nil is returned.
func (e *ExtensionResourceStateList) Get(kind string, name, purpose *string) *gardencorev1alpha1.ExtensionResourceState {
	for _, obj := range *e {
		if matchesExtensionResourceState(&obj, kind, name, purpose) {
			return &obj
		}
	}
	return nil
}

// Delete removes an ExtensionResourceState from the list by kind, name and purpose
func (e *ExtensionResourceStateList) Delete(kind string, name, purpose *string) {
	for i, obj := range *e {
		if matchesExtensionResourceState(&obj, kind, name, purpose) {
			*e = append((*e)[:i], (*e)[i+1:]...)
			return
		}
	}
}

// Upsert either inserts or updates an already existing ExtensionResourceState with kind, name and purpose in the list
func (e *ExtensionResourceStateList) Upsert(extensionResourceState *gardencorev1alpha1.ExtensionResourceState) {
	for i, obj := range *e {
		if matchesExtensionResourceState(&obj, extensionResourceState.Kind, extensionResourceState.Name, extensionResourceState.Purpose) {
			(*e)[i].State = extensionResourceState.State
			(*e)[i].Resources = extensionResourceState.Resources
			return
		}
	}
	*e = append(*e, *extensionResourceState)
}

func matchesExtensionResourceState(extensionResourceState *gardencorev1alpha1.ExtensionResourceState, kind string, name, purpose *string) bool {
	if extensionResourceState.Kind == kind && apiequality.Semantic.DeepEqual(extensionResourceState.Name, name) && apiequality.Semantic.DeepEqual(extensionResourceState.Purpose, purpose) {
		return true
	}
	return false
}

// GardenerResourceDataList is a list of GardenerResourceData
type GardenerResourceDataList []gardencorev1alpha1.GardenerResourceData

// Delete deletes an item from the list
func (g *GardenerResourceDataList) Delete(name string) {
	for i, e := range *g {
		if e.Name == name {
			*g = append((*g)[:i], (*g)[i+1:]...)
		}
	}
}

// Get returns the item from the list
func (g *GardenerResourceDataList) Get(name string) *gardencorev1alpha1.GardenerResourceData {
	for _, resourceDataEntry := range *g {
		if resourceDataEntry.Name == name {
			return &resourceDataEntry
		}
	}
	return nil
}

// Upsert inserts a new element or updates an existing one
func (g *GardenerResourceDataList) Upsert(data *gardencorev1alpha1.GardenerResourceData) {
	for i, obj := range *g {
		if obj.Name == data.Name {
			(*g)[i].Type = data.Type
			(*g)[i].Data = data.Data
			return
		}
	}
	*g = append(*g, *data)
}

// ResourceDataList is a list of ResourceData
type ResourceDataList []gardencorev1alpha1.ResourceData

// Delete deletes an item from the list
func (r *ResourceDataList) Delete(ref *autoscalingv1.CrossVersionObjectReference) {
	for i, obj := range *r {
		if apiequality.Semantic.DeepEqual(obj.CrossVersionObjectReference, *ref) {
			*r = append((*r)[:i], (*r)[i+1:]...)
		}
	}
}

// Get returns the item from the list
func (r *ResourceDataList) Get(ref *autoscalingv1.CrossVersionObjectReference) *gardencorev1alpha1.ResourceData {
	for _, obj := range *r {
		if apiequality.Semantic.DeepEqual(obj.CrossVersionObjectReference, *ref) {
			return &obj
		}
	}
	return nil
}

// Upsert inserts a new element or updates an existing one
func (r *ResourceDataList) Upsert(data *gardencorev1alpha1.ResourceData) {
	for i, obj := range *r {
		if apiequality.Semantic.DeepEqual(obj.CrossVersionObjectReference, data.CrossVersionObjectReference) {
			(*r)[i].Data = data.Data
			return
		}
	}
	*r = append(*r, *data)
}
