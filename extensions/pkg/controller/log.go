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

package controller

import (
	"fmt"

	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

// CreateEventLogger creates a Logger with keys and values from the given CreateEvent.
func CreateEventLogger(log logr.Logger, event event.CreateEvent) logr.Logger {
	return log.WithValues(CreateEventLogValues(event)...)
}

// UpdateEventLogger creates a Logger with keys and values from the given UpdateEvent.
func UpdateEventLogger(log logr.Logger, event event.UpdateEvent) logr.Logger {
	return log.WithValues(UpdateEventLogValues(event)...)
}

// DeleteEventLogger creates a Logger with keys and values from the given DeleteEvent.
func DeleteEventLogger(log logr.Logger, event event.DeleteEvent) logr.Logger {
	return log.WithValues(DeleteEventLogValues(event)...)
}

// GenericEventLogger creates a Logger with keys and values from the given GenericEvent.
func GenericEventLogger(log logr.Logger, event event.GenericEvent) logr.Logger {
	return log.WithValues(GenericEventLogValues(event)...)
}

// PrefixLogValues prefixes the keys of the given logValues with the given prefix.
func PrefixLogValues(prefix string, logValues []interface{}) []interface{} {
	if prefix == "" {
		return logValues
	}
	if logValues == nil {
		return logValues
	}

	out := make([]interface{}, 0, len(logValues))
	for i := 0; i < len(logValues); i += 2 {
		key := logValues[i]
		value := logValues[i+1]
		out = append(out, fmt.Sprintf("%s.%s", prefix, key), value)
	}
	return out
}

func eventObjectLogValues(meta metav1.Object, obj runtime.Object) []interface{} {
	var values []interface{}
	if meta != nil {
		values = append(values, MetaObjectLogValues(meta)...)
	}
	if obj != nil {
		values = append(values, RuntimeObjectLogValues(obj)...)
	}
	return values
}

// CreateEventLogValues extracts the log values from the given CreateEvent.
func CreateEventLogValues(event event.CreateEvent) []interface{} {
	return eventObjectLogValues(event.Meta, event.Object)
}

// DeleteEventLogValues extracts the log values from the given DeleteEvent.
func DeleteEventLogValues(event event.DeleteEvent) []interface{} {
	return append(eventObjectLogValues(event.Meta, event.Object), "delete-state-unknown", event.DeleteStateUnknown)
}

// GenericEventLogValues extracts the log values from the given GenericEvent.
func GenericEventLogValues(event event.GenericEvent) []interface{} {
	return eventObjectLogValues(event.Meta, event.Object)
}

// UpdateEventLogValues extracts the log values from the given UpdateEvent.
func UpdateEventLogValues(event event.UpdateEvent) []interface{} {
	var values []interface{}
	values = append(values, PrefixLogValues("old", eventObjectLogValues(event.MetaOld, event.ObjectOld))...)
	values = append(values, PrefixLogValues("new", eventObjectLogValues(event.MetaNew, event.ObjectNew))...)
	return values
}

// MetaObjectLogValues extracts the log values from the given metav1.Object.
func MetaObjectLogValues(obj metav1.Object) []interface{} {
	values := []interface{}{"meta.name", obj.GetName()}
	if namespace := obj.GetNamespace(); namespace != "" {
		values = append(values, "meta.namespace", namespace)
	}
	return values
}

// RuntimeObjectLogValues extracts the log values from the given runtime.Object.
func RuntimeObjectLogValues(obj runtime.Object) []interface{} {
	apiVersion, kind := obj.GetObjectKind().GroupVersionKind().ToAPIVersionAndKind()
	return []interface{}{
		"object.apiVersion", apiVersion,
		"object.kind", kind,
	}
}
