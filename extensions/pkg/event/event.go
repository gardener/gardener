// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
