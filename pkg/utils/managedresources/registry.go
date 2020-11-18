// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package managedresources

import (
	"fmt"
	"reflect"
	"strings"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
)

// Registry stores objects and their serialized form. It allows to compute a map of all registered objects that can be
// used as part of a Secret's data which is referenced by a ManagedResource.
type Registry struct {
	scheme       *runtime.Scheme
	codec        runtime.Codec
	nameToObject map[string]*object
}

type object struct {
	obj           runtime.Object
	serialization []byte
}

// NewRegistry returns a new registry for resources. The given scheme, codec, and serializer must know all the resource
// types that will later be added to the registry.
func NewRegistry(scheme *runtime.Scheme, codec serializer.CodecFactory, serializer *json.Serializer) *Registry {
	var groupVersions []schema.GroupVersion
	for k := range scheme.AllKnownTypes() {
		groupVersions = append(groupVersions, k.GroupVersion())
	}

	return &Registry{
		scheme:       scheme,
		codec:        codec.CodecForVersions(serializer, serializer, schema.GroupVersions(groupVersions), schema.GroupVersions(groupVersions)),
		nameToObject: make(map[string]*object),
	}
}

// Add adds the given object the registry. It computes a filename based on its type, namespace, and name. It serializes
// the object to YAML and stores both representations (object and serialization) in the registry.
func (r *Registry) Add(obj runtime.Object) error {
	if obj == nil || reflect.ValueOf(obj) == reflect.Zero(reflect.TypeOf(obj)) {
		return nil
	}

	objectName, err := r.objectName(obj)
	if err != nil {
		return err
	}
	filename := objectName + ".yaml"

	if _, ok := r.nameToObject[filename]; ok {
		return fmt.Errorf("duplicate filename in registry: %q", filename)
	}

	serializationYAML, err := runtime.Encode(r.codec, obj)
	if err != nil {
		return err
	}

	r.nameToObject[filename] = &object{
		obj:           obj,
		serialization: serializationYAML,
	}

	return nil
}

// SerializedObjects returns a map whose keys are filenames and whose values are serialized objects.
func (r *Registry) SerializedObjects() map[string][]byte {
	out := make(map[string][]byte, len(r.nameToObject))
	for name, object := range r.nameToObject {
		out[name] = object.serialization
	}
	return out
}

// AddAllAndSerialize calls Add() for all the given objects before calling SerializedObjects().
func (r *Registry) AddAllAndSerialize(objects ...runtime.Object) (map[string][]byte, error) {
	for _, resource := range objects {
		if err := r.Add(resource); err != nil {
			return nil, err
		}
	}
	return r.SerializedObjects(), nil
}

// RegisteredObjects returns a map whose keys are filenames and whose values are objects.
func (r *Registry) RegisteredObjects() map[string]runtime.Object {
	out := make(map[string]runtime.Object, len(r.nameToObject))
	for name, object := range r.nameToObject {
		out[name] = object.obj
	}
	return out
}

// String returns the string representation of the registry.
func (r *Registry) String() string {
	out := make([]string, 0, len(r.nameToObject))
	for name, object := range r.nameToObject {
		out = append(out, fmt.Sprintf("* %s:\n%s", name, object.serialization))
	}
	return strings.Join(out, "\n\n")
}

func (r *Registry) objectName(obj runtime.Object) (string, error) {
	gvk, err := apiutil.GVKForObject(obj, r.scheme)
	if err != nil {
		return "", err
	}

	acc, err := meta.Accessor(obj)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf(
		"%s__%s__%s",
		strings.ToLower(gvk.Kind),
		acc.GetNamespace(),
		strings.Replace(acc.GetName(), ":", "_", -1),
	), nil
}
