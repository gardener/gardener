// SPDX-FileCopyrightText: 2019 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package event

import (
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

// NewFromObject creates a new GenericEvent from the given runtime.Object.
//
// It tries to extract a metav1.Object from the given Object. If it fails, the Meta
// of the resulting GenericEvent will be `nil`.
func NewFromObject(obj runtime.Object) event.GenericEvent {
	accessor, err := meta.Accessor(obj)
	if err != nil {
		return NewGeneric(nil, obj)
	}

	return NewGeneric(accessor, obj)
}

// NewGeneric creates a new GenericEvent from the given metav1.Object and runtime.Object.
func NewGeneric(meta metav1.Object, obj runtime.Object) event.GenericEvent {
	return event.GenericEvent{
		Meta:   meta,
		Object: obj,
	}
}
