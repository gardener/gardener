// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package helper

import (
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/labels"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
)

// ExtensionResourceStateList is a list of ExtensionResourceStates
type ExtensionResourceStateList []gardencorev1beta1.ExtensionResourceState

// Get retrieves an ExtensionResourceState for given kind, name and purpose from a list of ExtensionResourceStates
// If no ExtensionResourceStates can be found, nil is returned.
func (e *ExtensionResourceStateList) Get(kind string, name, purpose *string) *gardencorev1beta1.ExtensionResourceState {
	for _, obj := range *e {
		if matchesExtensionResourceState(&obj, kind, name, purpose) {
			return &obj
		}
	}
	return nil
}

// Delete removes an ExtensionResourceState from the list by kind, name and purpose
func (e *ExtensionResourceStateList) Delete(kind string, name, purpose *string) {
	for i := len(*e) - 1; i >= 0; i-- {
		if matchesExtensionResourceState(&(*e)[i], kind, name, purpose) {
			*e = append((*e)[:i], (*e)[i+1:]...)
			return
		}
	}
}

// Upsert either inserts or updates an already existing ExtensionResourceState with kind, name and purpose in the list
func (e *ExtensionResourceStateList) Upsert(extensionResourceState *gardencorev1beta1.ExtensionResourceState) {
	for i, obj := range *e {
		if matchesExtensionResourceState(&obj, extensionResourceState.Kind, extensionResourceState.Name, extensionResourceState.Purpose) {
			(*e)[i].State = extensionResourceState.State
			(*e)[i].Resources = extensionResourceState.Resources
			return
		}
	}

	*e = append(*e, *extensionResourceState)
}

func matchesExtensionResourceState(extensionResourceState *gardencorev1beta1.ExtensionResourceState, kind string, name, purpose *string) bool {
	if extensionResourceState.Kind == kind && apiequality.Semantic.DeepEqual(extensionResourceState.Name, name) && apiequality.Semantic.DeepEqual(extensionResourceState.Purpose, purpose) {
		return true
	}
	return false
}

// GardenerResourceDataList is a list of GardenerResourceData
type GardenerResourceDataList []gardencorev1beta1.GardenerResourceData

// Delete deletes an item from the list
func (g *GardenerResourceDataList) Delete(name string) {
	for i := len(*g) - 1; i >= 0; i-- {
		if (*g)[i].Name == name {
			*g = append((*g)[:i], (*g)[i+1:]...)
			return
		}
	}
}

// Get returns the item from the list
func (g *GardenerResourceDataList) Get(name string) *gardencorev1beta1.GardenerResourceData {
	for _, resourceDataEntry := range *g {
		if resourceDataEntry.Name == name {
			return &resourceDataEntry
		}
	}
	return nil
}

// Select uses the provided label selector to find data entries with matching labels.
func (g *GardenerResourceDataList) Select(labelSelector labels.Selector) []*gardencorev1beta1.GardenerResourceData {
	var results []*gardencorev1beta1.GardenerResourceData

	for _, resourceDataEntry := range *g {
		if labelSelector.Matches(labels.Set(resourceDataEntry.Labels)) {
			results = append(results, resourceDataEntry.DeepCopy())
		}
	}

	return results
}

// Upsert inserts a new element or updates an existing one
func (g *GardenerResourceDataList) Upsert(data *gardencorev1beta1.GardenerResourceData) {
	for i, obj := range *g {
		if obj.Name == data.Name {
			(*g)[i].Type = data.Type
			(*g)[i].Data = data.Data
			(*g)[i].Labels = data.Labels
			return
		}
	}

	*g = append(*g, *data)
}

// DeepCopy makes a deep copy of a GardenerResourceDataList
func (g GardenerResourceDataList) DeepCopy() GardenerResourceDataList {
	res := GardenerResourceDataList{}
	for _, obj := range g {
		res = append(res, *obj.DeepCopy())
	}
	return res
}

// ResourceDataList is a list of ResourceData
type ResourceDataList []gardencorev1beta1.ResourceData

// Delete deletes an item from the list
func (r *ResourceDataList) Delete(ref *autoscalingv1.CrossVersionObjectReference) {
	for i := len(*r) - 1; i >= 0; i-- {
		if apiequality.Semantic.DeepEqual((*r)[i].CrossVersionObjectReference, *ref) {
			*r = append((*r)[:i], (*r)[i+1:]...)
			return
		}
	}
}

// Get returns the item from the list
func (r *ResourceDataList) Get(ref *autoscalingv1.CrossVersionObjectReference) *gardencorev1beta1.ResourceData {
	for _, obj := range *r {
		if apiequality.Semantic.DeepEqual(obj.CrossVersionObjectReference, *ref) {
			return &obj
		}
	}
	return nil
}

// Upsert inserts a new element or updates an existing one
func (r *ResourceDataList) Upsert(data *gardencorev1beta1.ResourceData) {
	for i, obj := range *r {
		if apiequality.Semantic.DeepEqual(obj.CrossVersionObjectReference, data.CrossVersionObjectReference) {
			(*r)[i].Data = data.Data
			return
		}
	}

	*r = append(*r, *data)
}
